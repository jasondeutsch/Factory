package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"factory/comms/internal/comms"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestClientReadOnlySnapshot(t *testing.T) {
	c, err := NewClient("http://localhost:8090/mcp", "secret")
	require.NoError(t, err)
	calls := 0
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		var data any
		switch r.URL.Path {
		case "/monitor/projects.json":
			data = map[string]any{"projects": []string{"/my project"}}
		case "/monitor/tasks.json":
			require.Equal(t, "/my project", r.URL.Query().Get("project"))
			require.Equal(t, "12", r.URL.Query().Get("before"))
			data = map[string]any{"tasks": []comms.MonitorTask{{Task: comms.Task{ID: "task1", Project: "/my project"}}}}
		case "/tasks/task1/status.json":
			data = comms.TaskStatus{Report: comms.TaskReport{Task: comms.Task{ID: "task1", Project: "/my project"}}}
		default:
			t.Fatalf("unexpected endpoint %s", r.URL.Path)
		}
		body, _ := json.Marshal(data)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	result, err := c.fetch(context.Background(), "/my project", "", 12)
	require.NoError(t, err)
	require.Equal(t, "task1", result.selected)
	require.Equal(t, 3, calls)
}
func TestFetchGenerationAndStaleData(t *testing.T) {
	c, _ := NewClient("http://localhost:8090", "")
	m := New(context.Background(), c, "project")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	require.True(t, m.busy)
	require.Equal(t, 1, m.generation)
	next, _ = m.Update(fetchedMsg{generation: 0, data: snapshot{selected: "wrong"}})
	m = next.(Model)
	require.Empty(t, m.selected)
	next, _ = m.Update(fetchedMsg{generation: 1, data: snapshot{selected: "right"}})
	m = next.(Model)
	require.Equal(t, "right", m.selected)
	next, _ = m.Update(fetchedMsg{generation: 1, err: errors.New("offline")})
	m = next.(Model)
	require.Equal(t, "right", m.selected)
	require.Contains(t, ansi.Strip(m.View()), "STALE")
	m.cancel()
}
func TestViewFitsAndSanitizesRemoteText(t *testing.T) {
	m := New(context.Background(), nil, "/project")
	task := comms.Task{ID: "test", Name: "Build hello world", Status: "working"}
	m.selected = task.ID
	m.data.tasks = []comms.MonitorTask{{Task: task, Factory: "golang", ActiveStage: "build"}}
	m.data.detail = &comms.TaskStatus{Report: comms.TaskReport{Task: task, MessageCount: 2}, Messages: []comms.Message{{Sender: "coordinator", Recipient: "coder", Status: "exec", Text: "Implement hello world"}, {Sender: "coder", Recipient: "reviewer", Status: "sent", Text: "Review please"}}}
	for _, size := range [][2]int{{120, 40}, {80, 24}, {65, 18}} {
		m.width, m.height = size[0], size[1]
		for tab := 0; tab < 3; tab++ {
			m.tab = tab
			view := m.View()
			require.LessOrEqual(t, len(strings.Split(view, "\n")), m.height)
			for _, line := range strings.Split(view, "\n") {
				require.LessOrEqual(t, ansi.StringWidth(line), m.width, "%q", line)
			}
		}
	}
	require.Equal(t, "danger", clean("\x1b[2Jdanger"))
	require.NotContains(t, clean("hello\r\nworld\a"), "\a")
	require.Contains(t, ansi.Strip(m.messageFlow(70)), "coder")
	require.Contains(t, ansi.Strip(m.messageFlow(70)), "EXEC 1")
}
