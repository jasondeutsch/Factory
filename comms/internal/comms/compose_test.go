//go:build integration

package comms

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The unique Compose project, image, network, and volume are removed on exit.
func TestComposeCommsAndAdminSharePersistentData(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	docker := os.Getenv("COMMS_TEST_DOCKER")
	if docker == "" {
		docker = "docker"
	}
	compose := []string{docker, "compose"}
	if executable := os.Getenv("COMMS_TEST_COMPOSE"); executable != "" {
		compose = []string{executable}
	}
	project := "factory-comms-test-" + strings.ToLower(rand.Text()[:8])
	environment := append(os.Environ(), "COMMS_PORT=0", "COMMS_ADMIN_PORT=0", "COMMS_AUTH_TOKEN=")
	base := append(append([]string{}, compose...), "-p", project, "-f", filepath.Join(root, "compose.yaml"))
	command := func(arguments ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		process := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
		process.Dir = root
		process.Env = environment
		output, err := process.CombinedOutput()
		return strings.TrimSpace(string(output)), err
	}
	run := func(prefix []string, arguments ...string) string {
		t.Helper()
		output, err := command(append(append([]string{}, prefix...), arguments...)...)
		require.NoError(t, err, output)
		return output
	}
	t.Cleanup(func() {
		if t.Failed() {
			output, _ := command(append(append([]string{}, base...), "logs", "--no-color", "--tail", "60")...)
			t.Log(output)
		}
		output, err := command(append(base, "down", "-v", "--rmi", "local", "--remove-orphans")...)
		assert.NoError(t, err, output)
	})
	run(base, "build")
	start := func(prefix []string) string {
		t.Helper()
		run(prefix, "up", "--detach", "--no-build")
		mapping := run(prefix, "port", "comms", "8080")
		endpoint := "http://" + mapping
		require.Eventually(t, func() bool {
			client := http.Client{Timeout: time.Second}
			response, err := client.Get(endpoint + "/healthz")
			if err != nil {
				return false
			}
			response.Body.Close()
			return response.StatusCode == http.StatusOK
		}, 15*time.Second, 100*time.Millisecond)
		container := run(prefix, "ps", "-q", "comms")
		run([]string{docker}, "exec", container, "/comms", "health")
		assert.Equal(t, "65532:65532", run([]string{docker}, "inspect", "--format", "{{.Config.User}}", container))
		return endpoint
	}
	call := func(endpoint, name string, input, output any) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		client := mcp.NewClient(&mcp.Implementation{Name: "compose-fixture", Version: "1"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint + "/mcp"}, nil)
		require.NoError(t, err)
		defer session.Close()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
		require.NoError(t, err)
		require.False(t, result.IsError, "%+v", result.Content)
		encoded, err := json.Marshal(result.StructuredContent)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(encoded, output))
	}
	endpoint := start(base)
	var task Task
	call(endpoint, "create_task", CreateTaskInput{Project: "container-fixture", Name: "Persistent", Brief: "Shared comms and admin database", Owner: "coordinator"}, &task)
	run(base, "restart", "comms")
	endpoint = start(base)
	var restored Task
	call(endpoint, "get_task", TaskInput{TaskID: task.ID}, &restored)
	assert.Equal(t, task.ID, restored.ID)
	run(base, "down")
	endpoint = start(base)
	call(endpoint, "get_task", TaskInput{TaskID: task.ID}, &restored)
	assert.Equal(t, task.ID, restored.ID)
	response, err := http.Get(endpoint + "/tasks/" + task.ID + "/report.html")
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, string(body), "Persistent")

	// The panel starts with the same Compose file and must see MCP writes,
	// including committed records still in the WAL, without changing task data.
	mapping := run(base, "port", "sqlite-admin", "8080")
	panel := "http://" + mapping
	require.Eventually(t, func() bool {
		client := http.Client{Timeout: time.Second}
		response, err := client.Get(panel + "/")
		if err != nil {
			return false
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		return err == nil && response.StatusCode == http.StatusOK && strings.Contains(string(body), "tasks")
	}, 20*time.Second, 100*time.Millisecond)
	call(endpoint, "create_task", CreateTaskInput{Project: "container-fixture", Name: "Live panel record", Brief: "Written through MCP while the panel is open", Owner: "coordinator"}, &restored)
	response, err = http.Get(panel + "/tasks/content/")
	require.NoError(t, err)
	body, err = io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, string(body), "Live panel record")
	assert.Contains(t, string(body), "Persistent")

	container := run(base, "ps", "-q", "sqlite-admin")
	assert.Equal(t, "65532:65532", run([]string{docker}, "inspect", "--format", "{{.Config.User}}", container))
	assert.Contains(t, run([]string{docker}, "inspect", "--format", "{{range .Mounts}}{{if eq .Destination \"/data\"}}{{.RW}}{{end}}{{end}}", container), "false")
	call(endpoint, "get_task", TaskInput{TaskID: task.ID}, &restored)
	assert.Equal(t, task.ID, restored.ID)

	run(base, "down", "-v")
	endpoint = start(base)
	var tasks taskList
	call(endpoint, "list_tasks", ListTasksInput{Project: "container-fixture"}, &tasks)
	assert.Empty(t, tasks.Tasks, "explicit volume removal resets task history")

}
