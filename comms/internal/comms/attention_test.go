package comms

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAttentionResolutionIsExplicitVersionedAndIdempotent(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "inbox")
	owner := testParticipant(t, s, task, "coordinator")
	worker := testParticipant(t, s, task, "worker")
	request := RequestAttentionInput{TaskID: task.ID, Actor: worker.ID, Recipient: "human", Kind: "approval", Title: "Approve spec", Detail: "Review the exact artifact hash", Options: []string{"approve", "reject"}, References: []string{"spec.html#sha256"}, RequestKey: "ask"}
	item, err := s.RequestAttention(t.Context(), request)
	require.NoError(t, err)
	again, err := s.RequestAttention(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, item.ID, again.ID)
	resolve := ResolveAttentionInput{TaskID: task.ID, ID: item.ID, Actor: worker.ID, ExpectedVersion: 1, Resolution: "approve", Explanation: "User approved", RequestKey: "resolve"}
	_, err = s.ResolveAttention(t.Context(), resolve)
	require.ErrorContains(t, err, "owner")
	resolve.Actor = owner.ID
	resolve.ExpectedVersion = 2
	_, err = s.ResolveAttention(t.Context(), resolve)
	require.ErrorContains(t, err, "stale")
	resolve.ExpectedVersion = 1
	resolved, err := s.ResolveAttention(t.Context(), resolve)
	require.NoError(t, err)
	assert.Equal(t, "resolved", resolved.State)
	_, err = s.ResolveAttention(t.Context(), resolve)
	require.NoError(t, err)
	inbox, err := s.InboxSummary(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Empty(t, inbox.Items)
	unchanged, err := s.Task(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.Status, unchanged.Status)
}
func TestSubmittedResultCreatesOneActionableReview(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "review-inbox")
	owner := testParticipant(t, s, task, "coordinator")
	worker := testParticipant(t, s, task, "worker")
	a := testAssignment(t, s, task, owner, worker)
	a = updateAssignment(t, s, a, worker, "accepted")
	input := SubmitResultInput{TaskID: task.ID, AssignmentID: a.ID, Actor: worker.ID, ExpectedVersion: a.Version, RequestKey: "result", Result: AssignmentResult{Outcome: "ready_for_review", Summary: "Ready", NextAction: "Independent review"}}
	_, err := s.SubmitResult(t.Context(), input)
	require.NoError(t, err)
	_, err = s.SubmitResult(t.Context(), input)
	require.NoError(t, err)
	inbox, err := s.InboxSummary(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	require.Len(t, inbox.Items, 1)
	assert.Equal(t, "review", inbox.Items[0].Kind)
	assert.Equal(t, []string{a.ID}, inbox.Items[0].References)
}
