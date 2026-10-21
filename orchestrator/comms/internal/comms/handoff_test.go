package comms

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHandoffRequirementsRejectMissingEvidenceWithoutPublishing(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "handoff")
	owner := testParticipant(t, s, task, "coordinator")
	worker := testParticipant(t, s, task, "worker")
	a, err := s.CreateAssignment(t.Context(), CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: worker.ID, Title: "Build", Instructions: "Scope", ExpectedOutput: []string{"source"}, Stage: "build", Attempt: 1, RequestKey: "assign", Handoff: HandoffRequirements{RequireArtifacts: true, RequireChecks: true, RequireLimitations: true, Artifacts: []string{"main.go"}, Checks: []string{"unit"}}})
	require.NoError(t, err)
	a = updateAssignment(t, s, a, worker, "accepted")
	in := SubmitResultInput{TaskID: task.ID, AssignmentID: a.ID, Actor: worker.ID, ExpectedVersion: a.Version, RequestKey: "submit", Result: AssignmentResult{Outcome: "ready_for_review", Summary: "Done", NextAction: "Review"}}
	_, err = s.SubmitResult(t.Context(), in)
	require.ErrorContains(t, err, "limitations")
	in.Result.Limitations = []string{}
	in.Result.Artifacts = []string{"main.go"}
	in.Result.Checks = []CheckEvidence{{Name: "lint", Command: "lint", ExitStatus: 0}}
	_, err = s.SubmitResult(t.Context(), in)
	require.ErrorContains(t, err, "unit")
	current, err := s.Assignment(t.Context(), AssignmentInput{TaskID: task.ID, AssignmentID: a.ID})
	require.NoError(t, err)
	assert.Equal(t, a.Version, current.Version)
	inbox, err := s.InboxSummary(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Empty(t, inbox.Items)
	in.Result.Checks = append(in.Result.Checks, CheckEvidence{Name: "unit", Command: "go test ./...", ExitStatus: 1})
	submitted, err := s.SubmitResult(t.Context(), in)
	require.NoError(t, err)
	assert.Equal(t, "submitted", submitted.State) // Evidence completeness is not a passing quality gate.
}
func TestFailureCanReportMissingArtifacts(t *testing.T) {
	err := validateResultEvidence(HandoffRequirements{RequireArtifacts: true, RequireChecks: true, Artifacts: []string{"main.go"}}, AssignmentResult{Outcome: "failed", Limitations: []string{"Compiler unavailable"}})
	require.NoError(t, err)
	err = validateResultEvidence(HandoffRequirements{}, AssignmentResult{Outcome: "failed"})
	require.Error(t, err)
}

func TestReviewerCanRequestChanges(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "review-changes")
	owner := testParticipant(t, s, task, "coordinator")
	reviewer := testParticipant(t, s, task, "reviewer")
	a, err := s.CreateAssignment(t.Context(), CreateAssignmentInput{TaskID: task.ID, AssigningParticipant: owner.ID, Assignee: reviewer.ID, Title: "Review", Instructions: "Inspect changes", ExpectedOutput: []string{"findings"}, Stage: "review", Attempt: 1, RequestKey: "assign"})
	require.NoError(t, err)
	a = updateAssignment(t, s, a, reviewer, "accepted")
	result, err := s.SubmitResult(t.Context(), SubmitResultInput{TaskID: task.ID, AssignmentID: a.ID, Actor: reviewer.ID, ExpectedVersion: a.Version, RequestKey: "review-result", Result: AssignmentResult{Outcome: "changes_requested", Summary: "Correct the edge case", NextAction: "Return to coder"}})
	require.NoError(t, err)
	assert.Equal(t, "changes_requested", result.Result.Outcome)
}
