package comms

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testParticipant(t *testing.T, store *Store, task Task, identity string) Participant {
	t.Helper()
	participant, err := store.RegisterParticipant(t.Context(), RegisterParticipantInput{TaskID: task.ID, InstanceID: "session-" + identity, Role: identity, RoutingIdentity: identity, RequestKey: "register-" + identity})
	require.NoError(t, err)
	return participant
}

func testAssignment(t *testing.T, store *Store, task Task, owner, worker Participant) Assignment {
	t.Helper()
	assignment, err := store.CreateAssignment(t.Context(), CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: worker.ID, Title: "Build <game>", Instructions: "Implement approved scope only", Inputs: []string{"spec.html", "plan.html"}, ExpectedOutput: []string{"source and tests", "reviews/build-01.md"}, Stage: "build", Attempt: 1, RequestKey: "assign-build"})
	require.NoError(t, err)
	return assignment
}

func updateAssignment(t *testing.T, store *Store, assignment Assignment, actor Participant, state string) Assignment {
	t.Helper()
	updated, err := store.UpdateAssignment(t.Context(), UpdateAssignmentInput{TaskID: assignment.TaskID, AssignmentID: assignment.ID, Actor: actor.ID, ExpectedVersion: assignment.Version, State: state, Summary: "State: " + state, RequestKey: fmt.Sprintf("update-%s-%d", state, assignment.Version)})
	require.NoError(t, err)
	return updated
}

func TestAssignmentLifecycleNotificationsAndResults(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "team")
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	assignment := testAssignment(t, store, task, owner, worker)
	assert.Equal(t, "offered", assignment.State)
	inbox, err := store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: worker.RoutingIdentity})
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, assignment.ID, inbox[0].AssignmentID)
	_, err = store.Acknowledge(t.Context(), AcknowledgeInput{TaskID: task.ID, MessageID: inbox[0].ID, Actor: worker.RoutingIdentity})
	require.NoError(t, err)
	assignment, err = store.Assignment(t.Context(), AssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID})
	require.NoError(t, err)
	assert.Equal(t, "offered", assignment.State, "reading is not acceptance")

	for _, state := range []string{"accepted", "working", "blocked", "needs_input", "working"} {
		assignment = updateAssignment(t, store, assignment, worker, state)
	}
	input := SubmitResultInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, RequestKey: "result-build", Result: AssignmentResult{Outcome: "ready_for_review", Summary: "Ready <script>bad</script>", Artifacts: []string{"reviews/build-01.md"}, Checks: []CheckEvidence{{Command: "go test ./...", ExitStatus: 0}}, Limitations: []string{"gopls unavailable"}, NextAction: "Assign independent review"}}
	submitted, err := store.SubmitResult(t.Context(), input)
	require.NoError(t, err)
	replayed, err := store.SubmitResult(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, submitted, replayed)
	assert.Equal(t, "submitted", submitted.State)
	require.NotNil(t, submitted.Result)
	assert.Equal(t, "ready_for_review", submitted.Result.Outcome)
	input.Result.Summary = "Different result"
	_, err = store.SubmitResult(t.Context(), input)
	require.ErrorContains(t, err, "different coordination request")
	inbox, err = store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: owner.RoutingIdentity})
	require.NoError(t, err)
	require.Len(t, inbox, 6, "each accepted update and submission notifies the orchestrator once")
	assert.Equal(t, "result", inbox[5].Kind)
	assert.Equal(t, assignment.ID, inbox[5].AssignmentID)
	report, err := store.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", report.Task.Status, "result submission does not complete the task")
	assert.Equal(t, 2, report.ParticipantCount)
	assert.Equal(t, 1, report.AssignmentCount)
	assert.Equal(t, 1, report.AssignmentCounts["submitted"])
	assert.Equal(t, "submitted", report.Stages[0].Status)
	assert.Equal(t, 1, report.EventCounts["assignment_created"])
	assert.Equal(t, 5, report.EventCounts["assignment_updated"])
	assert.Equal(t, 1, report.EventCounts["assignment_submitted"])
	var html bytes.Buffer
	require.NoError(t, RenderReport(&html, report))
	assert.Contains(t, html.String(), "gopls unavailable")
	assert.Contains(t, html.String(), "&lt;script&gt;")
	assert.Contains(t, html.String(), "Participants")
	assert.NotContains(t, html.String(), "<script>")
}

func TestAssignmentBoundariesTransitionsAndRetries(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "team")
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	peer := testParticipant(t, store, task, "reviewer")
	otherTask := testTask(t, store, "other")
	other := testParticipant(t, store, otherTask, "coder")
	assignment := testAssignment(t, store, task, owner, worker)
	same := testAssignment(t, store, task, owner, worker)
	assert.Equal(t, assignment, same)
	_, err := store.CreateAssignment(t.Context(), CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: other.ID, Title: "Cross task", Instructions: "Invalid", ExpectedOutput: []string{"result"}, Stage: "build", Attempt: 1, RequestKey: "cross-task"})
	require.ErrorContains(t, err, "participant not found")
	_, err = store.CreateAssignment(t.Context(), CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: peer.ID, Assignee: worker.ID, Title: "Unauthorized", Instructions: "Invalid", ExpectedOutput: []string{"result"}, Stage: "build", Attempt: 1, RequestKey: "non-owner"})
	require.ErrorContains(t, err, "task owner")
	_, err = store.Assignment(t.Context(), AssignmentInput{TaskID: otherTask.ID, AssignmentID: assignment.ID})
	require.ErrorContains(t, err, "not found")
	_, err = store.SendMessage(t.Context(), SendMessageInput{TaskID: otherTask.ID, AssignmentID: assignment.ID, Sender: "coder", Recipient: "coordinator", Kind: "question", Text: "Cross task"})
	require.ErrorContains(t, err, "not found")

	for _, test := range []struct {
		name, actor, state, errorText string
	}{
		{"skip acceptance", worker.ID, "working", "invalid assignment transition"},
		{"wrong actor", peer.ID, "accepted", "declared assignee"},
		{"cross-task actor", other.ID, "accepted", "participant not found"},
		{"worker cancellation", worker.ID, "cancelled", "task owner"},
		{"bypass result", worker.ID, "submitted", "submit_result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.UpdateAssignment(t.Context(), UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: test.actor, ExpectedVersion: assignment.Version, State: test.state, Summary: "Invalid", RequestKey: test.name})
			require.ErrorContains(t, err, test.errorText)
		})
	}
	result := SubmitResultInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, RequestKey: "early-result", Result: AssignmentResult{Outcome: "completed", Summary: "Not accepted", NextAction: "Review"}}
	_, err = store.SubmitResult(t.Context(), result)
	require.ErrorContains(t, err, "cannot submit")
	accepted := updateAssignment(t, store, assignment, worker, "accepted")
	_, err = store.UpdateAssignment(t.Context(), UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: 1, State: "working", Summary: "Stale", RequestKey: "stale"})
	require.ErrorContains(t, err, "stale")
	// Replaying the original acceptance returns its original response, even after later updates.
	working := updateAssignment(t, store, accepted, worker, "working")
	replayed := updateAssignment(t, store, assignment, worker, "accepted")
	assert.Equal(t, accepted, replayed)
	result.Actor = peer.ID
	result.ExpectedVersion = working.Version
	_, err = store.SubmitResult(t.Context(), result)
	require.ErrorContains(t, err, "declared assignee")
	cancelled := updateAssignment(t, store, working, owner, "cancelled")
	result.Actor = worker.ID
	result.ExpectedVersion = cancelled.Version
	_, err = store.SubmitResult(t.Context(), result)
	require.ErrorContains(t, err, "cannot submit")
	items, err := store.Assignments(t.Context(), ListAssignmentsInput{TaskID: task.ID, Assignee: worker.ID, State: "cancelled", Limit: 1})
	require.NoError(t, err)
	require.Len(t, items, 1)
	items, err = store.Assignments(t.Context(), ListAssignmentsInput{TaskID: task.ID, After: items[0].ID})
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestCoordinationRollbackDoesNotLoseNotifications(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "atomic")
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	_, err := store.db.Exec("CREATE TRIGGER reject_notification BEFORE INSERT ON messages BEGIN SELECT RAISE(ABORT,'notification unavailable'); END")
	require.NoError(t, err)
	input := CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: worker.ID, Title: "Atomic", Instructions: "One item", ExpectedOutput: []string{"result"}, Stage: "build", Attempt: 1, RequestKey: "atomic-create"}
	_, err = store.CreateAssignment(t.Context(), input)
	require.ErrorContains(t, err, "notification unavailable")
	items, err := store.Assignments(t.Context(), ListAssignmentsInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Empty(t, items)
	report, err := store.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Zero(t, report.MessageCount)
	assert.Zero(t, report.EventCounts["assignment_created"])
	_, err = store.db.Exec("DROP TRIGGER reject_notification")
	require.NoError(t, err)
	assignment, err := store.CreateAssignment(t.Context(), input)
	require.NoError(t, err)
	_, err = store.db.Exec("CREATE TRIGGER reject_event BEFORE INSERT ON events WHEN NEW.kind='assignment_updated' BEGIN SELECT RAISE(ABORT,'event unavailable'); END")
	require.NoError(t, err)
	update := UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: 1, State: "accepted", Summary: "Accepted", RequestKey: "atomic-accept"}
	_, err = store.UpdateAssignment(t.Context(), update)
	require.ErrorContains(t, err, "event unavailable")
	current, err := store.Assignment(t.Context(), AssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID})
	require.NoError(t, err)
	assert.Equal(t, assignment, current)
	inbox, err := store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: owner.RoutingIdentity})
	require.NoError(t, err)
	assert.Empty(t, inbox)
	_, err = store.db.Exec("DROP TRIGGER reject_event")
	require.NoError(t, err)
	_, err = store.UpdateAssignment(t.Context(), update)
	require.NoError(t, err)
}

func TestConcurrentAssignmentVersionsAndResultReplay(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "concurrency")
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	assignment := updateAssignment(t, store, testAssignment(t, store, task, owner, worker), worker, "accepted")
	var group sync.WaitGroup
	outcomes := make(chan error, 12)
	for index := range 12 {
		group.Go(func() {
			_, err := store.UpdateAssignment(t.Context(), UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, State: "working", Summary: fmt.Sprintf("Update %d", index), RequestKey: fmt.Sprintf("concurrent-%d", index)})
			outcomes <- err
		})
	}
	group.Wait()
	close(outcomes)
	successes := 0
	for err := range outcomes {
		if err == nil {
			successes++
		} else {
			require.ErrorContains(t, err, "stale")
		}
	}
	assert.Equal(t, 1, successes)
	assignment, err := store.Assignment(t.Context(), AssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID})
	require.NoError(t, err)
	input := SubmitResultInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, RequestKey: "concurrent-result", Result: AssignmentResult{Outcome: "completed", Summary: "Assignment complete", NextAction: "Owner reviews"}}
	outcomes = make(chan error, 12)
	for range 12 {
		group.Go(func() {
			_, err := store.SubmitResult(t.Context(), input)
			outcomes <- err
		})
	}
	group.Wait()
	close(outcomes)
	for err := range outcomes {
		require.NoError(t, err)
	}
	report, err := store.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, report.EventCounts["assignment_submitted"])
	assert.Equal(t, 4, report.MessageCount, "offered, accepted, working, and one result")
}

func TestParticipantInstancesReplacementAndPagination(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "participants")
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	input := RegisterParticipantInput{TaskID: task.ID, InstanceID: "second-coder", Role: worker.Role, RoutingIdentity: "coder-2", RequestKey: "register"}
	second, err := store.RegisterParticipant(t.Context(), input)
	require.NoError(t, err)
	assert.NotEqual(t, worker.ID, second.ID)
	replayed, err := store.RegisterParticipant(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, second, replayed)
	input.RoutingIdentity = "different"
	_, err = store.RegisterParticipant(t.Context(), input)
	require.ErrorContains(t, err, "different coordination request")
	input.RequestKey = "duplicate-route"
	input.RoutingIdentity = "coder"
	input.InstanceID = "third-coder"
	_, err = store.RegisterParticipant(t.Context(), input)
	require.ErrorContains(t, err, "already registered")
	assignment := updateAssignment(t, store, testAssignment(t, store, task, owner, worker), worker, "accepted")
	input.ReplacesParticipantID = worker.ID
	input.RequestKey = "replacement"
	replacement, err := store.RegisterParticipant(t.Context(), input)
	require.NoError(t, err)
	assert.NotEqual(t, worker.ID, replacement.ID)
	_, err = store.UpdateAssignment(t.Context(), UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, State: "working", Summary: "Stale worker", RequestKey: "stale-worker"})
	require.ErrorContains(t, err, "retired")
	_, err = store.SubmitResult(t.Context(), SubmitResultInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: replacement.ID, ExpectedVersion: assignment.Version, RequestKey: "inherit-result", Result: AssignmentResult{Outcome: "completed", Summary: "Not my assignment", NextAction: "Review"}})
	require.ErrorContains(t, err, "declared assignee")
	newOwner, err := store.RegisterParticipant(t.Context(), RegisterParticipantInput{TaskID: task.ID, InstanceID: "new-coordinator", Role: owner.Role, RoutingIdentity: owner.RoutingIdentity, ReplacesParticipantID: owner.ID, RequestKey: "new-owner"})
	require.NoError(t, err)
	cancelled := updateAssignment(t, store, assignment, newOwner, "cancelled")
	assert.Equal(t, "cancelled", cancelled.State)
	page, err := store.Participants(t.Context(), ListParticipantsInput{TaskID: task.ID, Limit: 2})
	require.NoError(t, err)
	require.Len(t, page, 2)
	require.NotNil(t, page[0].RetiredAt)
	require.NotNil(t, page[1].RetiredAt)
	next, err := store.Participants(t.Context(), ListParticipantsInput{TaskID: task.ID, Limit: 2, After: page[1].ID})
	require.NoError(t, err)
	require.Len(t, next, 2)
	assert.NotEqual(t, page[1].ID, next[0].ID)
}

func TestSchemaOneUpgradePreservesExistingEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	require.NoError(t, err)
	task := testTask(t, store, "old-schema")
	message, err := store.SendMessage(t.Context(), SendMessageInput{TaskID: task.ID, Sender: "architect", Recipient: "coordinator", Kind: "result", Text: "Old evidence"})
	require.NoError(t, err)
	// Restore exactly the original table shape/version before reopening.
	_, err = store.db.Exec("DROP TABLE deliveries; ALTER TABLE messages DROP COLUMN assignment_id; DROP TABLE coordination_requests; DROP TABLE assignments; DROP TABLE participants; ALTER TABLE messages DROP COLUMN executed_at; PRAGMA user_version=1;")
	require.NoError(t, err)
	require.NoError(t, store.Close())
	store, err = Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	restored, err := store.Task(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, task, restored)
	inbox, err := store.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: "coordinator"})
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, message, inbox[0])
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	assert.NotEmpty(t, testAssignment(t, store, task, owner, worker).ID)
}

func TestAssignmentDeclineAndTerminalTaskGuards(t *testing.T) {
	for _, terminal := range []string{"complete", "cancelled"} {
		t.Run(terminal, func(t *testing.T) {
			store := testStore(t)
			task := testTask(t, store, "terminal")
			owner := testParticipant(t, store, task, task.Owner)
			worker := testParticipant(t, store, task, "coder")
			assignment := updateAssignment(t, store, testAssignment(t, store, task, owner, worker), worker, "accepted")
			_, err := store.UpdateTask(t.Context(), UpdateTaskInput{TaskID: task.ID, Actor: task.Owner, ExpectedVersion: task.Version, Status: terminal, Summary: "Owner ended task"})
			require.NoError(t, err)
			_, err = store.UpdateAssignment(t.Context(), UpdateAssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, State: "working", Summary: "Continue", RequestKey: "continue"})
			require.ErrorContains(t, err, "task is complete or cancelled")
			_, err = store.SubmitResult(t.Context(), SubmitResultInput{TaskID: task.ID, AssignmentID: assignment.ID, Actor: worker.ID, ExpectedVersion: assignment.Version, RequestKey: "late-result", Result: AssignmentResult{Outcome: "completed", Summary: "Late", NextAction: "Review"}})
			require.ErrorContains(t, err, "task is complete or cancelled")
			_, err = store.CreateAssignment(t.Context(), CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: worker.ID, Title: "More work", Instructions: "Invalid", ExpectedOutput: []string{"result"}, Stage: "build", Attempt: 2, RequestKey: "more-work"})
			require.ErrorContains(t, err, "cannot assign")
			assert.Equal(t, "cancelled", updateAssignment(t, store, assignment, owner, "cancelled").State)
		})
	}
	store := testStore(t)
	task := testTask(t, store, "decline")
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	declined := updateAssignment(t, store, testAssignment(t, store, task, owner, worker), worker, "declined")
	_, err := store.UpdateAssignment(t.Context(), UpdateAssignmentInput{TaskID: task.ID, AssignmentID: declined.ID, Actor: worker.ID, ExpectedVersion: declined.Version, State: "accepted", Summary: "Resurrect", RequestKey: "resurrect"})
	require.ErrorContains(t, err, "invalid assignment transition")
}

func TestBindParticipantPreservesIdentityAndAssignment(t *testing.T) {
	store := testStore(t)
	task := testTask(t, store, "team")
	owner := testParticipant(t, store, task, task.Owner)
	worker := testParticipant(t, store, task, "coder")
	peer := testParticipant(t, store, task, "reviewer")
	assignment := testAssignment(t, store, task, owner, worker)
	input := BindParticipantInput{TaskID: task.ID, ParticipantID: worker.ID, Actor: owner.ID, SessionRef: `{"adapter":"herdr","name":"coder"}`, RequestKey: "bind-coder"}
	bound, err := store.BindParticipant(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, worker.ID, bound.ID)
	assert.Equal(t, worker.InstanceID, bound.InstanceID)
	assert.Nil(t, bound.RetiredAt)
	assert.Equal(t, input.SessionRef, bound.SessionRef)
	replayed, err := store.BindParticipant(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, bound, replayed)
	current, err := store.Assignment(t.Context(), AssignmentInput{TaskID: task.ID, AssignmentID: assignment.ID})
	require.NoError(t, err)
	assert.Equal(t, assignment, current)

	input.RequestKey = "stale-bind"
	input.SessionRef = "changed"
	_, err = store.BindParticipant(t.Context(), input)
	require.ErrorContains(t, err, "stale session binding")
	input.ExpectedSessionRef = bound.SessionRef
	input.Actor = peer.ID
	_, err = store.BindParticipant(t.Context(), input)
	require.ErrorContains(t, err, "only the participant")
	input.Actor = worker.ID
	input.SessionRef = ""
	input.RequestKey = "disable-delivery"
	detached, err := store.BindParticipant(t.Context(), input)
	require.NoError(t, err)
	assert.Empty(t, detached.SessionRef)
	_, err = store.RegisterParticipant(t.Context(), RegisterParticipantInput{TaskID: task.ID, InstanceID: "replacement", Role: worker.Role, RoutingIdentity: worker.RoutingIdentity, ReplacesParticipantID: worker.ID, RequestKey: "replace"})
	require.NoError(t, err)
	input.Actor, input.RequestKey, input.ExpectedSessionRef = owner.ID, "retired-bind", ""
	_, err = store.BindParticipant(t.Context(), input)
	require.ErrorContains(t, err, "retired")
	input.TaskID = testTask(t, store, "other").ID
	_, err = store.BindParticipant(t.Context(), input)
	require.ErrorContains(t, err, "not found")
}
