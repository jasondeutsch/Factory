package comms

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRecoveryPreservesEvidenceAndAtomicallyCreatesNewWork(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "recover")
	owner := testParticipant(t, s, task, "coordinator")
	old := testParticipant(t, s, task, "old")
	replacement := testParticipant(t, s, task, "new")
	a := testAssignment(t, s, task, owner, old)
	a = updateAssignment(t, s, a, old, "accepted")
	plan, err := s.RecoveryPlan(t.Context(), AssignmentInput{TaskID: task.ID, AssignmentID: a.ID})
	require.NoError(t, err)
	assert.Contains(t, plan.Actions, "reassign")
	in := RecoverAssignmentInput{TaskID: task.ID, AssignmentID: a.ID, Actor: owner.ID, ExpectedVersion: a.Version, Action: "reassign", ReplacementID: replacement.ID, Reason: "Operator stopped failed worker", RequestKey: "recover"}
	_, err = s.RecoverAssignment(t.Context(), in)
	require.ErrorContains(t, err, "confirmation")
	in.PreviousSessionStopped = true
	in.ReplacementID = "missing"
	_, err = s.RecoverAssignment(t.Context(), in)
	require.Error(t, err)
	unchanged, err := s.Assignment(t.Context(), AssignmentInput{TaskID: task.ID, AssignmentID: a.ID})
	require.NoError(t, err)
	assert.Equal(t, a.Version, unchanged.Version)
	in.ReplacementID = replacement.ID
	out, err := s.RecoverAssignment(t.Context(), in)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", out.Previous.State)
	assert.Equal(t, a.Summary, out.Previous.Summary)
	require.NotNil(t, out.Replacement)
	assert.Equal(t, replacement.ID, out.Replacement.Assignee)
	assert.Contains(t, out.Replacement.Inputs, "assignment:"+a.ID)
	again, err := s.RecoverAssignment(t.Context(), in)
	require.NoError(t, err)
	assert.Equal(t, out.Replacement.ID, again.Replacement.ID)
	all, err := s.Assignments(t.Context(), ListAssignmentsInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Len(t, all, 2)
	in.RequestKey = "stale"
	_, err = s.RecoverAssignment(t.Context(), in)
	require.ErrorContains(t, err, "stale")
}
