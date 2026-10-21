package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Client struct{ session *mcp.ClientSession }
type tokenTransport struct{ token string }

func (t tokenTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(copy)
}
func Connect(ctx context.Context, url, token string) (*Client, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	// Do not forward bearer credentials to a redirect target.
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("comms redirects are not allowed") }
	if token != "" {
		httpClient.Transport = tokenTransport{token}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "factory-delivery", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: httpClient}, nil)
	if err != nil {
		return nil, err
	}
	return &Client{session}, nil
}
func (c *Client) Close() error { return c.session.Close() }
func (c *Client) Call(ctx context.Context, name string, input, output any) error {
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
	if err != nil {
		return err
	}
	if result.IsError {
		return fmt.Errorf("comms %s rejected request", name)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return err
	}
	if string(encoded) == "null" {
		return fmt.Errorf("comms %s returned no structured result", name)
	}
	return json.Unmarshal(encoded, output)
}
