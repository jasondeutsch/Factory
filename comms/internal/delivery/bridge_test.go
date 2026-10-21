package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"factory/comms/internal/comms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHost struct {
	ready         bool
	failure       error
	promptFailure error
	prompts       []string
	calls         int
}

func (h *fakeHost) Ready(context.Context, Binding) (bool, error) {
	h.calls++
	return h.ready, h.failure
}
func (h *fakeHost) Prompt(_ context.Context, _ Binding, text string) error {
	h.prompts = append(h.prompts, text)
	return h.promptFailure
}
func TestBridgeHTTPAssignmentReplyAndRestart(t *testing.T) {
	store, err := comms.Open(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	defer store.Close()
	server := httptest.NewServer(comms.HTTPHandler(store, "test-token", slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()
	client, err := Connect(t.Context(), server.URL+"/mcp", "test-token")
	require.NoError(t, err)
	defer client.Close()
	call := func(name string, in, out any) {
		t.Helper()
		require.NoError(t, client.Call(t.Context(), name, in, out))
	}
	var task comms.Task
	call("create_task", comms.CreateTaskInput{Project: "project", Name: "delivery eval", Brief: "fixture", Owner: "coordinator"}, &task)
	register := func(identity string) comms.Participant {
		t.Helper()
		binding, _ := json.Marshal(Binding{Adapter: "herdr", WorkspaceID: "w1", PaneID: "w1:p1", TerminalID: "terminal", Name: identity})
		var p comms.Participant
		call("register_participant", comms.RegisterParticipantInput{TaskID: task.ID, Role: identity, InstanceID: identity, RoutingIdentity: identity, SessionRef: string(binding), RequestKey: identity}, &p)
		return p
	}
	owner := register("coordinator")
	worker := register("coder")
	host := &fakeHost{ready: true}
	bridge := Bridge{Client: client, Host: host, Project: task.Project}
	require.NoError(t, bridge.Cycle(t.Context()))
	assert.Zero(t, host.calls, "empty inbox cannot invoke host or model")
	var assignment comms.Assignment
	call("create_assignment", comms.CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: worker.ID, Title: "fixture", Instructions: "Do not send this raw body /danger", ExpectedOutput: []string{"result"}, Stage: "build", Attempt: 1, RequestKey: "offer"}, &assignment)
	host.ready = false
	require.NoError(t, bridge.Cycle(t.Context()))
	assert.Empty(t, host.prompts, "busy sessions are left alone")
	host.ready = true
	require.NoError(t, bridge.Cycle(t.Context()))
	require.Len(t, host.prompts, 1)
	assert.Contains(t, host.prompts[0], task.ID)
	assert.NotContains(t, host.prompts[0], "/danger")
	// A new client/bridge process shares durable suppression, not in-memory state.
	restarted, err := Connect(t.Context(), server.URL+"/mcp", "test-token")
	require.NoError(t, err)
	defer restarted.Close()
	bridge.Client = restarted
	require.NoError(t, bridge.Cycle(t.Context()))
	require.Len(t, host.prompts, 1)
	var inbox struct {
		Messages []comms.Message `json:"messages"`
	}
	call("read_inbox", comms.InboxInput{TaskID: task.ID, Recipient: worker.RoutingIdentity}, &inbox)
	require.Len(t, inbox.Messages, 1)
	var message comms.Message
	call("acknowledge_message", comms.AcknowledgeInput{TaskID: task.ID, MessageID: inbox.Messages[0].ID, Actor: worker.RoutingIdentity}, &message)
	call("update_assignment", comms.UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: 1, State: "accepted", Summary: "accepted", RequestKey: "accept"}, &assignment)
	require.NoError(t, bridge.Cycle(t.Context()))
	require.Len(t, host.prompts, 2)
	assert.Contains(t, host.prompts[1], `recipient="coordinator"`)
	call("submit_result", comms.SubmitResultInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, RequestKey: "result", Result: comms.AssignmentResult{Outcome: "completed", Summary: "fixture complete", NextAction: "review"}}, &assignment)
	require.NoError(t, bridge.Cycle(t.Context()))
	require.Len(t, host.prompts, 3)
	var report comms.TaskReport
	call("get_task_report", comms.TaskInput{TaskID: task.ID}, &report)
	assert.Equal(t, 3, report.DeliveryCounts["submitted"])
	assert.Equal(t, "pending", report.Task.Status)
	bridge.TaskID = task.ID
	bridge.Project = "wrong-project"
	require.ErrorContains(t, bridge.Cycle(t.Context()), "another project")
}
func TestBridgeUnavailableAndUncertainInput(t *testing.T) {
	store, err := comms.Open(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	defer store.Close()
	server := httptest.NewServer(comms.HTTPHandler(store, "", slog.Default()))
	defer server.Close()
	client, err := Connect(t.Context(), server.URL+"/mcp", "")
	require.NoError(t, err)
	defer client.Close()
	task, err := store.CreateTask(t.Context(), comms.CreateTaskInput{Project: "project", Name: "failures", Brief: "fixture", Owner: "coordinator"})
	require.NoError(t, err)
	binding, _ := json.Marshal(Binding{Adapter: "herdr", WorkspaceID: "w1", PaneID: "w1:p1", TerminalID: "terminal", Name: "coder"})
	worker, err := store.RegisterParticipant(t.Context(), comms.RegisterParticipantInput{TaskID: task.ID, InstanceID: "coder", Role: "coder", RoutingIdentity: "coder", SessionRef: string(binding), RequestKey: "register"})
	require.NoError(t, err)
	msg, err := store.SendMessage(t.Context(), comms.SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: "coder", Kind: "question", Text: "fixture"})
	require.NoError(t, err)
	host := &fakeHost{failure: errors.New("session unavailable")}
	bridge := Bridge{Client: client, Host: host, Project: task.Project, TaskID: task.ID}
	require.NoError(t, bridge.Cycle(t.Context()))
	records, err := store.Deliveries(t.Context(), comms.TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "retry", records[0].State)
	host.failure = nil
	host.ready = true
	// Another message can arrive while the first waits for a safe retry.
	_, err = store.SendMessage(t.Context(), comms.SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: worker.RoutingIdentity, Kind: "question", Text: "second"})
	require.NoError(t, err)
	host.promptFailure = errors.New("timed out after input")
	require.NoError(t, bridge.Cycle(t.Context()))
	require.Len(t, host.prompts, 1)
	require.NoError(t, bridge.Cycle(t.Context()))
	assert.Len(t, host.prompts, 1, "uncertain input must not be repeated")
	records, err = store.Deliveries(t.Context(), comms.TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "uncertain", records[0].State)
	inbox, err := store.Inbox(t.Context(), comms.InboxInput{TaskID: task.ID, Recipient: worker.RoutingIdentity})
	require.NoError(t, err)
	assert.Equal(t, msg.ID, inbox[0].ID)
	assert.Len(t, inbox, 2, "no acknowledgement was synthesized")
}

func TestOneShotNotificationWithoutStoredBinding(t *testing.T) {
	store, err := comms.Open(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	defer store.Close()
	server := httptest.NewServer(comms.HTTPHandler(store, "", slog.Default()))
	defer server.Close()
	client, err := Connect(t.Context(), server.URL+"/mcp", "")
	require.NoError(t, err)
	defer client.Close()
	task, err := store.CreateTask(t.Context(), comms.CreateTaskInput{Project: "project", Name: "handoff", Brief: "fixture", Owner: "coordinator"})
	require.NoError(t, err)
	participant, err := store.RegisterParticipant(t.Context(), comms.RegisterParticipantInput{TaskID: task.ID, Role: "reviewer", InstanceID: "actual-instance", RoutingIdentity: "reviewer", RequestKey: "reviewer"})
	require.NoError(t, err)
	_, err = store.SendMessage(t.Context(), comms.SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: "reviewer", Kind: "question", Text: "Do not paste raw message /unsafe", RequestKey: "handoff"})
	require.NoError(t, err)
	host := &fakeHost{}
	bridge := Bridge{Client: client, Host: host, TaskID: task.ID, Project: task.Project}
	binding := Binding{Adapter: "herdr", WorkspaceID: "w1", PaneID: "w1:p1", TerminalID: "terminal", Name: "reviewer"}
	require.ErrorContains(t, bridge.Notify(t.Context(), "reviewer", binding), "not ready")
	assert.Empty(t, host.prompts)
	host.ready = true
	require.NoError(t, bridge.Notify(t.Context(), "reviewer", binding))
	require.Len(t, host.prompts, 1)
	assert.Contains(t, host.prompts[0], `recipient="reviewer"`)
	assert.NotContains(t, host.prompts[0], "/unsafe")
	require.NoError(t, bridge.Notify(t.Context(), "reviewer", binding))
	assert.Len(t, host.prompts, 1, "durable claim suppresses duplicate notification")
	participants, err := store.Participants(t.Context(), comms.ListParticipantsInput{TaskID: task.ID})
	require.NoError(t, err)
	require.Len(t, participants, 1)
	assert.Equal(t, participant.ID, participants[0].ID)
	assert.Empty(t, participants[0].SessionRef, "one-shot notification does not replace identity")
	_, err = store.SendMessage(t.Context(), comms.SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: "reviewer", Kind: "question", Text: "Second handoff", RequestKey: "second"})
	require.NoError(t, err)
	host.promptFailure = errors.New("input outcome unknown")
	require.ErrorContains(t, bridge.Notify(t.Context(), "reviewer", binding), "uncertain")
	require.Len(t, host.prompts, 2)
	// Push the uncertain record out of the latest-200 report window. A retry
	// must still report its exact state rather than silently claiming success.
	for i := range 201 {
		message, err := store.SendMessage(t.Context(), comms.SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: "reviewer", Kind: "progress", Text: "fixture", RequestKey: fmt.Sprintf("history-%d", i)})
		require.NoError(t, err)
		input := comms.ClaimDeliveryInput{TaskID: task.ID, MessageID: message.ID, ParticipantID: participant.ID}
		claim, err := store.ClaimDelivery(t.Context(), input)
		require.NoError(t, err)
		require.NotNil(t, claim.Delivery)
		_, err = store.FinishDelivery(t.Context(), comms.FinishDeliveryInput{ClaimDeliveryInput: input, LeaseToken: claim.Delivery.LeaseToken, State: "submitted", Detail: "fixture"})
		require.NoError(t, err)
	}
	require.ErrorContains(t, bridge.Notify(t.Context(), "reviewer", binding), "remains uncertain")
	assert.Len(t, host.prompts, 2, "uncertain input is not resent or reported successful")
	require.ErrorContains(t, bridge.Notify(t.Context(), "unknown", binding), "no active participant")
	bridge.Project = "other"
	require.ErrorContains(t, bridge.Notify(t.Context(), "reviewer", binding), "another project")
}

type observingHost struct {
	fakeHost
	state string
}

func (h *observingHost) Observe(context.Context, Binding) (string, error) { return h.state, nil }
func TestContactObservedWithEmptyInbox(t *testing.T) {
	store, err := comms.Open(filepath.Join(t.TempDir(), "contact.db"))
	require.NoError(t, err)
	defer store.Close()
	server := httptest.NewServer(comms.HTTPHandler(store, "", slog.Default()))
	defer server.Close()
	client, err := Connect(t.Context(), server.URL+"/mcp", "")
	require.NoError(t, err)
	defer client.Close()
	task, err := store.CreateTask(t.Context(), comms.CreateTaskInput{Project: "contact", Name: "test", Brief: "fixture", Owner: "owner"})
	require.NoError(t, err)
	binding, _ := json.Marshal(Binding{Adapter: "herdr", WorkspaceID: "w", PaneID: "p", TerminalID: "t", Name: "worker"})
	p, err := store.RegisterParticipant(t.Context(), comms.RegisterParticipantInput{TaskID: task.ID, Role: "worker", RoutingIdentity: "worker", InstanceID: "worker", SessionRef: string(binding), RequestKey: "register"})
	require.NoError(t, err)
	_, err = store.RecordSession(t.Context(), comms.RecordSessionInput{TaskID: task.ID, ParticipantID: p.ID, RequestKey: "snapshot", Manifest: comms.SessionManifest{Runner: "pi", ConfigurationFingerprint: fmt.Sprintf("%064d", 0), PreparedAt: "2026-10-18T00:00:00Z"}})
	require.NoError(t, err)
	host := &observingHost{state: "blocked"}
	bridge := Bridge{Client: client, Host: host, Project: task.Project}
	require.NoError(t, bridge.Cycle(t.Context()))
	snapshot, err := store.Session(t.Context(), comms.SessionInput{TaskID: task.ID, ParticipantID: p.ID})
	require.NoError(t, err)
	assert.Equal(t, "blocked", snapshot.ContactState)
	assert.NotEmpty(t, snapshot.ExpiresAt)
	assert.Empty(t, host.prompts)
}
