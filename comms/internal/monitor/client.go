// Package monitor implements a read-only terminal client for Comms.
package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"factory/comms/internal/comms"
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

func NewClient(endpoint, token string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("endpoint must be an HTTP(S) Comms URL without credentials, query or fragment")
	}
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/mcp")
	return &Client{base: strings.TrimRight(u.String(), "/"), token: token, http: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) get(ctx context.Context, route string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+route, nil)
	if err != nil {
		return errors.New("could not prepare monitor request")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return errors.New("Comms unreachable; check endpoint, network and service")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return errors.New("authentication required: set COMMS_AUTH_TOKEN and restart")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Comms returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(target); err != nil {
		return errors.New("invalid or oversized Comms response")
	}
	return nil
}

type snapshot struct {
	projects []string
	tasks    []comms.MonitorTask
	detail   *comms.TaskStatus
	selected string
}

func (c *Client) fetch(ctx context.Context, project, selected string, before int64) (snapshot, error) {
	var result snapshot
	var projects struct {
		Projects []string `json:"projects"`
	}
	if err := c.get(ctx, "/monitor/projects.json", &projects); err != nil {
		return result, err
	}
	result.projects = projects.Projects
	var tasks struct {
		Tasks []comms.MonitorTask `json:"tasks"`
	}
	route := fmt.Sprintf("/monitor/tasks.json?project=%s&limit=50&before=%d", url.QueryEscape(project), before)
	if err := c.get(ctx, route, &tasks); err != nil {
		return result, err
	}
	result.tasks = tasks.Tasks
	for _, item := range tasks.Tasks {
		if item.Task.ID == selected {
			result.selected = selected
		}
	}
	if result.selected == "" && len(tasks.Tasks) > 0 {
		result.selected = tasks.Tasks[0].Task.ID
	}
	if result.selected != "" {
		result.detail = new(comms.TaskStatus)
		if err := c.get(ctx, "/tasks/"+url.PathEscape(result.selected)+"/status.json", result.detail); err != nil {
			return result, err
		}
		if result.detail.Report.Task.ID != result.selected || result.detail.Report.Task.Project != project {
			return result, errors.New("Comms returned a mismatched task; view not updated")
		}
	}
	return result, nil
}
