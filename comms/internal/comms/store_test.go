package comms

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

func testTask(t *testing.T, store *Store, project string) Task {
	t.Helper()
	task, err := store.CreateTask(t.Context(), CreateTaskInput{Project: project, Name: "Mixed CASE Task", Brief: "Keep it small.", Owner: "coordinator"})
	require.NoError(t, err)
	return task
}

func TestTaskIsolationOrderingAndStatusConflicts(t *testing.T) {
	store := testStore(t)
	first := testTask(t, store, "project-a")
	second := testTask(t, store, "project-a")
	testTask(t, store, "project-b")
	page, err := store.Tasks(t.Context(), ListTasksInput{Project: "project-a", Limit: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, second.ID, page[0].ID)
	page, err = store.Tasks(t.Context(), ListTasksInput{Project: "project-a", Before: page[0].Sequence})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, first.ID, page[0].ID)
	input := UpdateTaskInput{TaskID: first.ID, Actor: "coder", ExpectedVersion: 1, Status: "working", Summary: "Implementing"}
	_, err = store.UpdateTask(t.Context(), input)
	require.ErrorContains(t, err, "owner")
	input.Actor = "coordinator"
	updated, err := store.UpdateTask(t.Context(), input)
	require.NoError(t, err)
	assert.EqualValues(t, 2, updated.Version)
	_, err = store.UpdateTask(t.Context(), input)
	require.ErrorContains(t, err, "stale")
	input.ExpectedVersion = 2
	input.Status = "invented"
	_, err = store.UpdateTask(t.Context(), input)
	require.Error(t, err)
	events, err := store.Events(t.Context(), ListEventsInput{TaskID: first.ID})
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "status_changed", events[1].Kind)
}

func TestRetryKeysAndRecipientAcknowledgements(t *testing.T) {
	store := testStore(t)
	creation := CreateTaskInput{Project: "team", Name: "One task", Brief: "A brief", Owner: "coordinator", RequestKey: "creation-1"}
	task, err := store.CreateTask(t.Context(), creation)
	require.NoError(t, err)
	same, err := store.CreateTask(t.Context(), creation)
	require.NoError(t, err)
	assert.Equal(t, task.ID, same.ID)
	creation.Brief = "different"
	_, err = store.CreateTask(t.Context(), creation)
	require.ErrorContains(t, err, "different task")
	input := SendMessageInput{TaskID: task.ID, Sender: "architect", Recipient: "reviewer", Kind: "question", Text: "Review this.", RequestKey: "question-1"}
	message, err := store.SendMessage(t.Context(), input)
	require.NoError(t, err)
	sameMessage, err := store.SendMessage(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, message.ID, sameMessage.ID)
	input.Text = "Different question"
	_, err = store.SendMessage(t.Context(), input)
	require.ErrorContains(t, err, "different message")
	inbox, err := store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: "reviewer"})
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	_, err = store.Acknowledge(t.Context(), AcknowledgeInput{TaskID: task.ID, MessageID: message.ID, Actor: "coder"})
	require.ErrorContains(t, err, "recipient")
	other := testTask(t, store, "other")
	_, err = store.Acknowledge(t.Context(), AcknowledgeInput{TaskID: other.ID, MessageID: message.ID, Actor: "reviewer"})
	require.ErrorIs(t, err, errMessageNotFound)
	ack, err := store.Acknowledge(t.Context(), AcknowledgeInput{TaskID: task.ID, MessageID: message.ID, Actor: "reviewer"})
	require.NoError(t, err)
	again, err := store.Acknowledge(t.Context(), AcknowledgeInput{TaskID: task.ID, MessageID: message.ID, Actor: "reviewer"})
	require.NoError(t, err)
	assert.Equal(t, ack.AcknowledgedAt, again.AcknowledgedAt)
	inbox, err = store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: "reviewer"})
	require.NoError(t, err)
	assert.Empty(t, inbox)
	events, err := store.Events(t.Context(), ListEventsInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Len(t, events, 3)
}

func TestConcurrentMessagesAndReplay(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "concurrent")
	var group sync.WaitGroup
	failures := make(chan error, 40)
	for index := range 40 {
		group.Go(func() {
			_, err := store.SendMessage(context.Background(), SendMessageInput{TaskID: task.ID, Sender: "coder", Recipient: "coordinator", Kind: "progress", Text: fmt.Sprintf("Item %d", index%20), RequestKey: fmt.Sprintf("item-%d", index%20)})
			if err != nil {
				failures <- err
			}
		})
	}
	group.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	first, err := store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: "coordinator", Limit: 10})
	require.NoError(t, err)
	require.Len(t, first, 10)
	second, err := store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: "coordinator", Limit: 10, After: first[9].ID})
	require.NoError(t, err)
	require.Len(t, second, 10)
	for _, message := range first {
		for _, other := range second {
			assert.NotEqual(t, message.ID, other.ID)
		}
	}
	report, err := store.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, 20, report.MessageCount)
	assert.Equal(t, 21, report.EventCount)
}

func TestDatabaseSurvivesReopenAndFreshDatabaseStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	require.NoError(t, err)
	task := testTask(t, store, "durable")
	require.NoError(t, store.Close())
	store, err = Open(path)
	require.NoError(t, err)
	reopened, err := store.Task(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, task, reopened)
	require.NoError(t, store.Close())
	fresh := testStore(t)
	_, err = fresh.Task(t.Context(), task.ID)
	require.ErrorIs(t, err, errTaskNotFound)
}

func intValue(value int64) *int64      { return &value }
func costValue(value float64) *float64 { return &value }

func TestUsageCoverageRetriesAndEscapedReport(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "report")
	_, err := store.RecordEvent(t.Context(), RecordEventInput{TaskID: task.ID, Actor: "coder", Kind: "stage_started", Stage: "build", Attempt: 1, Message: "Started"})
	require.NoError(t, err)
	input := RecordUsageInput{TaskID: task.ID, Actor: "coder", Stage: "build", Attempt: 1, RequestKey: "provider-request-1", Usage: Usage{Provider: "custom", Model: "model-a", Source: "runtime", InputTokens: intValue(100), OutputTokens: intValue(20), CacheReadTokens: intValue(40), CostUSD: costValue(.01)}}
	usage, err := store.RecordUsage(t.Context(), input)
	require.NoError(t, err)
	replayed, err := store.RecordUsage(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, usage.Sequence, replayed.Sequence)
	report, err := store.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 120, *report.Usage.TotalTokens)
	assert.InDelta(t, .01, *report.Usage.CostUSD, .00001)
	input.Usage.OutputTokens = intValue(21)
	_, err = store.RecordUsage(t.Context(), input)
	require.ErrorContains(t, err, "different event")
	input.RequestKey = "request-2"
	input.Stage = "review"
	input.Usage.InputTokens = nil
	input.Usage.OutputTokens = intValue(10)
	input.Usage.CostUSD = nil
	_, err = store.RecordUsage(t.Context(), input)
	require.NoError(t, err)
	_, err = store.RecordEvent(t.Context(), RecordEventInput{TaskID: task.ID, Actor: "coder", Kind: "failure", Stage: "build", Attempt: 1, Message: "<script>alert('bad')</script>", Data: map[string]any{"tool": "go test", "mcp": "gopls"}})
	require.NoError(t, err)
	report, err = store.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Nil(t, report.Usage.TotalTokens)
	assert.EqualValues(t, 100, *report.Usage.InputTokens)
	assert.EqualValues(t, 30, *report.Usage.OutputTokens)
	assert.Equal(t, 2, report.Usage.Records)
	assert.Equal(t, 1, report.Usage.InputRecords)
	assert.Equal(t, 1, report.EventCounts["failure"])
	var output bytes.Buffer
	require.NoError(t, RenderReport(&output, report))
	assert.Contains(t, output.String(), "&lt;script&gt;")
	assert.NotContains(t, output.String(), "<script>")
	assert.Contains(t, output.String(), "Unknown")
	assert.Contains(t, output.String(), "gopls")
	assert.Contains(t, output.String(), "model-a")
	input.Usage.InputTokens = intValue(-1)
	_, err = store.RecordUsage(t.Context(), input)
	require.Error(t, err)
}

func TestEventPaginationAndInvalidReferences(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "events")
	input := RecordEventInput{TaskID: task.ID, Actor: "architect", Kind: "decision", Message: "Approved", RequestKey: "decision-1", Data: map[string]any{"outcome": "approve", "artifacts": []any{map[string]any{"path": "spec.html", "sha256": "declared-hash"}}}}
	first, err := store.RecordEvent(t.Context(), input)
	require.NoError(t, err)
	same, err := store.RecordEvent(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, first.Sequence, same.Sequence)
	events, err := store.Events(t.Context(), ListEventsInput{TaskID: task.ID, After: first.Sequence})
	require.NoError(t, err)
	assert.Empty(t, events)
	input.TaskID = "missing"
	_, err = store.RecordEvent(t.Context(), input)
	require.Error(t, err)
	input.TaskID = task.ID
	input.Kind = "usage"
	_, err = store.RecordEvent(t.Context(), input)
	require.ErrorContains(t, err, "invalid event kind")
	_, err = store.Events(t.Context(), ListEventsInput{TaskID: task.ID, Limit: 501})
	require.Error(t, err)
}
