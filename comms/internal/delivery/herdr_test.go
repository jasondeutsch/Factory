package delivery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHerdrAdapterUsesExplicitBindingAndKnownReadiness(t *testing.T) {
	t.Setenv("HERDR_ENV", "1") // Only the fixture executable below; never the real CLI.
	directory := t.TempDir()
	responsePath := filepath.Join(directory, "response.json")
	argsPath := filepath.Join(directory, "args.json")
	executable := filepath.Join(directory, "herdr-fixture")
	// Node.js is already a Factory prerequisite. This fixture has no Herdr socket.
	script := "#!/usr/bin/env node\nconst fs=require('node:fs');fs.writeFileSync(" + jsString(argsPath) + ",JSON.stringify(process.argv.slice(2)));console.log(fs.readFileSync(" + jsString(responsePath) + ",'utf8'));\n"
	require.NoError(t, os.WriteFile(executable, []byte(script), 0700))
	host := Herdr{Binary: executable}
	binding := Binding{Adapter: "herdr", WorkspaceID: "w1", PaneID: "w1:p2", TerminalID: "terminal-2", Name: "coder"}
	agent := agentInfo{WorkspaceID: binding.WorkspaceID, PaneID: binding.PaneID, TerminalID: binding.TerminalID, Name: binding.Name, Agent: "pi", Status: "idle"}
	write := func(kind string) {
		t.Helper()
		var response hostResponse
		response.Result.Type = kind
		response.Result.Agent = agent
		body, err := json.Marshal(response)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(responsePath, body, 0600))
	}
	write("agent_info")
	captured, err := host.Bind(t.Context(), "coder")
	require.NoError(t, err)
	assert.Equal(t, binding, captured)
	ready, err := host.Ready(t.Context(), binding)
	require.NoError(t, err)
	assert.True(t, ready)
	for _, status := range []string{"working", "blocked", "unknown"} {
		agent.Status = status
		write("agent_info")
		ready, err = host.Ready(t.Context(), binding)
		require.NoError(t, err)
		assert.False(t, ready)
	}
	agent.Status = "done"
	write("agent_info")
	ready, err = host.Ready(t.Context(), binding)
	require.NoError(t, err)
	assert.True(t, ready)
	negative := false
	agent.InteractiveReady = &negative
	write("agent_info")
	ready, err = host.Ready(t.Context(), binding)
	require.NoError(t, err)
	assert.False(t, ready)
	agent.InteractiveReady = nil
	agent.LaunchPending = true
	write("agent_info")
	ready, err = host.Ready(t.Context(), binding)
	require.NoError(t, err)
	assert.False(t, ready)
	agent.LaunchPending = false
	agent.TerminalID = "replacement"
	write("agent_info")
	_, err = host.Ready(t.Context(), binding)
	require.ErrorContains(t, err, "binding changed")
	agent.TerminalID = binding.TerminalID
	agent.WorkspaceID = "other"
	write("agent_info")
	_, err = host.Ready(t.Context(), binding)
	require.ErrorContains(t, err, "binding changed")
	agent.WorkspaceID = binding.WorkspaceID
	write("agent_prompted")
	text := "check inbox; $(never-a-shell)\nextra"
	require.NoError(t, host.Prompt(t.Context(), binding, text))
	var args []string
	body, err := os.ReadFile(argsPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, &args))
	assert.Equal(t, []string{"agent", "prompt", binding.PaneID, text}, args)
	t.Setenv("HERDR_ENV", "")
	_, err = host.Bind(t.Context(), "coder")
	require.ErrorContains(t, err, "Herdr-managed")
}
func jsString(value string) string { encoded, _ := json.Marshal(value); return string(encoded) }
func TestHerdrErrorsAndMalformedReferences(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	_, err := (Herdr{Binary: "/does/not/exist"}).Bind(t.Context(), "coder")
	require.ErrorContains(t, err, "failed")
	_, err = (Herdr{}).Bind(t.Context(), "--focused")
	require.Error(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = (Herdr{Binary: "/does/not/exist"}).Bind(ctx, "coder")
	require.Error(t, err)
	require.Error(t, (Binding{Adapter: "herdr", Name: "coder", WorkspaceID: "w1", PaneID: "--danger", TerminalID: "terminal"}).Validate())
}

func TestHerdrTimeoutAndStructuredFailure(t *testing.T) {
	t.Setenv("HERDR_ENV", "1") // Isolated fixture executable only.
	fixture := filepath.Join(t.TempDir(), "herdr-fixture")
	require.NoError(t, os.WriteFile(fixture, []byte("#!/usr/bin/env node\nconsole.error(JSON.stringify({id:'test',error:{code:'agent_not_found',message:'private content'}}));process.exit(1);\n"), 0700))
	_, err := (Herdr{Binary: fixture}).Bind(t.Context(), "coder")
	require.ErrorContains(t, err, "agent_not_found")
	assert.NotContains(t, err.Error(), "private content")
	require.NoError(t, os.WriteFile(fixture, []byte("#!/usr/bin/env node\nsetTimeout(()=>{},10000);\n"), 0700))
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = (Herdr{Binary: fixture}).Bind(ctx, "coder")
	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second)
}
