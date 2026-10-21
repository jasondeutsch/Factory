package comms

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPMCPRoundTripAndReport(t *testing.T) {
	store := testStore(t)
	server := httptest.NewServer(HTTPHandler(store, "", slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "comms-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp"}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { session.Close() })
	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 37)
	call := func(name string, input, output any) {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
		require.NoError(t, err)
		require.False(t, result.IsError, "%+v", result.Content)
		encoded, err := json.Marshal(result.StructuredContent)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(encoded, output))
	}
	var task Task
	call("create_task", CreateTaskInput{Project: "fixture", Name: "HTTP task", Brief: "<script>unsafe</script>", Owner: "coordinator"}, &task)
	var tasks taskList
	call("list_tasks", ListTasksInput{Project: "fixture"}, &tasks)
	require.Len(t, tasks.Tasks, 1)
	var report TaskReport
	call("get_task_report", TaskInput{TaskID: task.ID}, &report)
	assert.Nil(t, report.Usage.TotalTokens)
	var message Message
	call("send_message", SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: "coder", Kind: "assignment", Text: "Implement one item", RequestKey: "assignment-1"}, &message)
	var inbox messageList
	call("read_inbox", InboxInput{TaskID: task.ID, Recipient: "coder"}, &inbox)
	require.Len(t, inbox.Messages, 1)
	call("acknowledge_message", AcknowledgeInput{TaskID: task.ID, MessageID: message.ID, Actor: "coder"}, &message)
	var event Event
	call("record_event", RecordEventInput{TaskID: task.ID, Actor: "coder", Kind: "stage_started", Stage: "build", Attempt: 1, Message: "Started"}, &event)
	call("record_usage", RecordUsageInput{TaskID: task.ID, Actor: "coder", Stage: "build", Attempt: 1, RequestKey: "usage-1", Usage: Usage{Provider: "fixture", Model: "local", Source: "provider", InputTokens: intValue(10), OutputTokens: intValue(2)}}, &event)
	call("record_event", RecordEventInput{TaskID: task.ID, Actor: "coder", Kind: "stage_completed", Stage: "build", Attempt: 1, Message: "Finished"}, &event)
	call("update_task", UpdateTaskInput{TaskID: task.ID, Actor: "coordinator", ExpectedVersion: task.Version, Status: "complete", Summary: "Accepted"}, &task)
	call("get_task_report", TaskInput{TaskID: task.ID}, &report)
	require.NotNil(t, report.Usage.TotalTokens)
	assert.EqualValues(t, 12, *report.Usage.TotalTokens)
	// Exercise every coordination operation through actual MCP schemas/HTTP.
	var owner, worker Participant
	call("register_participant", RegisterParticipantInput{TaskID: task.ID, InstanceID: "owner-instance", Role: "orchestrator", RoutingIdentity: task.Owner, RequestKey: "register-owner"}, &owner)
	call("register_participant", RegisterParticipantInput{TaskID: task.ID, InstanceID: "worker-instance", Role: "coder", RoutingIdentity: "worker", RequestKey: "register-worker"}, &worker)
	call("bind_participant", BindParticipantInput{TaskID: task.ID, ParticipantID: worker.ID, Actor: worker.ID, SessionRef: "captured-host-reference", RequestKey: "bind-http"}, &worker)
	assert.Equal(t, "captured-host-reference", worker.SessionRef)
	var roster participantList
	call("list_participants", ListParticipantsInput{TaskID: task.ID}, &roster)
	require.Len(t, roster.Participants, 2)
	// Explicitly reopen the task; the service does not assign work on terminal tasks.
	call("update_task", UpdateTaskInput{TaskID: task.ID, Actor: task.Owner, ExpectedVersion: task.Version, Status: "working", Summary: "Coordination fixture"}, &task)
	var assignment Assignment
	call("create_assignment", CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: worker.ID, Title: "One assignment", Instructions: "Report evidence", ExpectedOutput: []string{"result"}, Stage: "coordination", Attempt: 1, RequestKey: "assignment-http"}, &assignment)
	call("get_assignment", AssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID}, &assignment)
	var listed assignmentList
	call("list_assignments", ListAssignmentsInput{TaskID: task.ID, State: "offered", Assignee: worker.ID}, &listed)
	require.Len(t, listed.Assignments, 1)
	call("update_assignment", UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, State: "accepted", Summary: "Accepted", RequestKey: "accept-http"}, &assignment)
	call("submit_result", SubmitResultInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, RequestKey: "result-http", Result: AssignmentResult{Outcome: "ready_for_review", Summary: "Ready", Artifacts: []string{"review.html"}, Checks: []CheckEvidence{{Command: "fixture check", ExitStatus: 0}}, NextAction: "Review the evidence"}}, &assignment)
	assert.Equal(t, "submitted", assignment.State)
	call("get_task_report", TaskInput{TaskID: task.ID}, &report)
	assert.Equal(t, 1, report.AssignmentCounts["submitted"])
	assert.Equal(t, "working", report.Task.Status)

	var rendered renderedReport
	call("render_report", TaskInput{TaskID: task.ID}, &rendered)
	assert.Contains(t, rendered.HTML, "&lt;script&gt;")
	response, err := http.Get(server.URL + report.ReportPath)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Usage by stage")
	assert.NotContains(t, string(body), "<script>")
	bad, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_task", Arguments: TaskInput{TaskID: "missing"}})
	require.NoError(t, err)
	assert.True(t, bad.IsError)
	response, err = http.Get(server.URL + "/tasks/missing/report.html")
	require.NoError(t, err)
	response.Body.Close()
	assert.Equal(t, http.StatusNotFound, response.StatusCode)
}

func TestHTTPAccessBoundaries(t *testing.T) {
	handler := HTTPHandler(testStore(t), "fixture-token", slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, test := range []struct {
		path, authorization, origin string
		status                      int
	}{
		{"/healthz", "", "", http.StatusOK},
		{"/monitor", "", "", http.StatusOK},
		{"/monitor.js", "", "", http.StatusOK},
		{"/monitor/projects.json", "", "", http.StatusUnauthorized},
		{"/mcp", "", "", http.StatusUnauthorized},
		{"/mcp", "Bearer wrong", "", http.StatusUnauthorized},
		{"/tasks/missing/report.html", "Bearer fixture-token", "", http.StatusNotFound},
		{"/mcp", "Bearer fixture-token", "https://elsewhere.example", http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set("Authorization", test.authorization)
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, test.status, response.Code, test.path)
	}
}

func TestProjectMonitorEndpoints(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "monitor-project")
	owner := testParticipant(t, store, task, "coordinator")
	worker := testParticipant(t, store, task, "builder")
	_, err := store.SetTaskFactory(t.Context(), SetTaskFactoryInput{TaskID: task.ID, Factory: FactorySnapshot{Name: "go-factory", Owner: "coordinator", Agents: []FactoryMember{{Name: "coordinator", Path: "orchestrator"}, {Name: "builder", Path: "agents/builder"}}}})
	require.NoError(t, err)
	assignment := testAssignment(t, store, task, owner, worker)
	assignment = updateAssignment(t, store, assignment, worker, "accepted")
	assignment = updateAssignment(t, store, assignment, worker, "working")
	_, err = store.RequestAttention(t.Context(), RequestAttentionInput{TaskID: task.ID, Actor: worker.ID, Recipient: "human", Kind: "approval", Title: "Review plan", Detail: "Plan is ready", RequestKey: "review-plan"})
	require.NoError(t, err)
	_, err = store.RecordEvent(t.Context(), RecordEventInput{TaskID: task.ID, Actor: "coordinator", Kind: "progress", Message: "Planning started"})
	require.NoError(t, err)
	_, err = store.SendMessage(t.Context(), SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: "builder", Kind: "assignment", Text: "Start work"})
	require.NoError(t, err)
	handler := HTTPHandler(store, "fixture-token", slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/monitor/projects.json", nil)
	request.Header.Set("Authorization", "Bearer fixture-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	var projects projectList
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &projects))
	assert.Equal(t, []string{"monitor-project"}, projects.Projects)

	request = httptest.NewRequest(http.MethodGet, "/monitor/tasks.json?project=monitor-project", nil)
	request.Header.Set("Authorization", "Bearer fixture-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	var tasks monitorTaskList
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &tasks))
	require.Len(t, tasks.Tasks, 1)
	assert.Equal(t, task.ID, tasks.Tasks[0].Task.ID)
	assert.Equal(t, "go-factory", tasks.Tasks[0].Factory)
	assert.Equal(t, "build", tasks.Tasks[0].ActiveStage)
	assert.Equal(t, assignment.Title, tasks.Tasks[0].ActiveAssignment)
	assert.Equal(t, "working", tasks.Tasks[0].AssignmentState)
	assert.Equal(t, "builder", tasks.Tasks[0].Assignee)
	assert.Equal(t, 1, tasks.Tasks[0].OpenAttention)
	assert.Equal(t, 4, tasks.Tasks[0].UnreadMessages)
	assert.Contains(t, tasks.Tasks[0].LatestEvent, "message_sent")

	request = httptest.NewRequest(http.MethodGet, "/monitor/tasks.json?project=", nil)
	request.Header.Set("Authorization", "Bearer fixture-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}
