// Package delivery delivers stored inbox hints; it never executes a workflow.
package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Binding is captured by an operator in Herdr, then recorded as session_ref.
// A name alone is insufficient: it could refer to a different workspace/pane.
type Binding struct {
	Adapter     string `json:"adapter"`
	WorkspaceID string `json:"workspace_id"`
	PaneID      string `json:"pane_id"`
	TerminalID  string `json:"terminal_id"`
	Name        string `json:"name"`
}
type agentInfo struct {
	WorkspaceID      string `json:"workspace_id"`
	PaneID           string `json:"pane_id"`
	TerminalID       string `json:"terminal_id"`
	Name             string `json:"name"`
	Agent            string `json:"agent"`
	Status           string `json:"agent_status"`
	InteractiveReady *bool  `json:"interactive_ready"`
	LaunchPending    bool   `json:"launch_pending"`
}
type hostResponse struct {
	Result struct {
		Type  string    `json:"type"`
		Agent agentInfo `json:"agent"`
	} `json:"result"`
}
type Herdr struct{ Binary string }

func (h Herdr) command(ctx context.Context, args ...string) (hostResponse, error) {
	var response hostResponse
	if os.Getenv("HERDR_ENV") != "1" {
		return response, errors.New("delivery must run inside a Herdr-managed pane (HERDR_ENV=1)")
	}
	deadline, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	binary := h.Binary
	if binary == "" {
		binary = "herdr"
	}
	// Arguments are passed directly, never interpreted by a shell. Do not include
	// terminal output in service records: it may contain credentials or user text.
	output, err := exec.CommandContext(deadline, binary, args...).Output()
	if err != nil {
		var exitError *exec.ExitError
		var failure struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if errors.As(err, &exitError) && json.Unmarshal(exitError.Stderr, &failure) == nil && errorCode.MatchString(failure.Error.Code) {
			return response, fmt.Errorf("herdr %s rejected notification/session operation (%s); inspect the target in Herdr", args[1], failure.Error.Code)
		}
		return response, fmt.Errorf("herdr %s failed: %w (inspect the target in Herdr)", args[1], err)
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return response, fmt.Errorf("decode Herdr response: %w", err)
	}
	return response, nil
}

var errorCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var agentName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func (b Binding) Validate() error {
	if b.Adapter != "herdr" || b.WorkspaceID == "" || (b.PaneID == "" || strings.HasPrefix(b.PaneID, "-") || strings.ContainsAny(b.PaneID, " \t\r\n")) || b.TerminalID == "" || !agentName.MatchString(b.Name) {
		return errors.New("session_ref needs a captured Herdr binding; use bridge --bind <actual-agent-name> inside Herdr")
	}
	return nil
}
func (h Herdr) Bind(ctx context.Context, name string) (Binding, error) {
	if !agentName.MatchString(name) {
		return Binding{}, errors.New("bind requires an explicit live Herdr agent name")
	}
	response, err := h.command(ctx, "agent", "get", name)
	if err != nil {
		return Binding{}, err
	}
	a := response.Result.Agent
	b := Binding{Adapter: "herdr", WorkspaceID: a.WorkspaceID, PaneID: a.PaneID, TerminalID: a.TerminalID, Name: a.Name}
	if response.Result.Type != "agent_info" || a.Agent == "" || a.Name != name {
		return Binding{}, errors.New("target is not a known agent")
	}
	return b, b.Validate()
}

// Ready refuses busy, blocked, unknown, replaced or moved sessions. It does not
// interrupt agents or answer dialogs. Missing optional readiness is supported
// for older Herdr versions, but a known negative value always prevents input.
func (h Herdr) Observe(ctx context.Context, b Binding) (string, error) {
	if err := b.Validate(); err != nil {
		return "unavailable", err
	}
	response, err := h.command(ctx, "agent", "get", b.PaneID)
	if err != nil {
		return "unavailable", err
	}
	a := response.Result.Agent
	if response.Result.Type != "agent_info" || a.Agent == "" || a.Name != b.Name || a.WorkspaceID != b.WorkspaceID || a.PaneID != b.PaneID || a.TerminalID != b.TerminalID {
		return "unavailable", errors.New("Herdr session binding changed; inspect the participant before replacing it")
	}
	switch a.Status {
	case "working", "blocked":
		return a.Status, nil
	case "idle", "done":
		if !a.LaunchPending && (a.InteractiveReady == nil || *a.InteractiveReady) {
			return "idle", nil
		}
	}
	return "unknown", nil
}
func (h Herdr) Ready(ctx context.Context, b Binding) (bool, error) {
	state, err := h.Observe(ctx, b)
	return state == "idle", err
}

func (h Herdr) Prompt(ctx context.Context, b Binding, text string) error {
	if err := b.Validate(); err != nil {
		return err
	}
	response, err := h.command(ctx, "agent", "prompt", b.PaneID, text)
	if err != nil {
		return err
	}
	a := response.Result.Agent
	if response.Result.Type != "agent_prompted" || a.Agent == "" || a.Name != b.Name || a.WorkspaceID != b.WorkspaceID || a.PaneID != b.PaneID || a.TerminalID != b.TerminalID {
		return errors.New("Herdr did not confirm agent_prompted for the bound recipient")
	}
	return nil
}
