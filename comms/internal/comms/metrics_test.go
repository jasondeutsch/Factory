package comms

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCoordinationMetricsUseRecordedClosedIntervals(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "metrics")
	owner := testParticipant(t, s, task, "coordinator")
	worker := testParticipant(t, s, task, "worker")
	a := testAssignment(t, s, task, owner, worker)
	empty, err := s.Metrics(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Nil(t, empty.Acceptance.MeanSeconds)
	assert.Equal(t, 1, empty.PendingAssignments)
	a = updateAssignment(t, s, a, worker, "accepted")
	a = updateAssignment(t, s, a, worker, "blocked")
	m, err := s.Metrics(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Zero(t, m.Blocked.Samples)
	updateAssignment(t, s, a, worker, "working")
	_, err = s.db.Exec(`UPDATE events SET created_at=CASE json_extract(data,'$.state') WHEN 'offered' THEN '2026-10-18T12:00:00Z' WHEN 'accepted' THEN '2026-10-18T12:00:05Z' WHEN 'blocked' THEN '2026-10-18T12:00:08Z' WHEN 'working' THEN '2026-10-18T12:00:18Z' END WHERE task_id=? AND kind IN ('assignment_created','assignment_updated')`, task.ID)
	require.NoError(t, err)
	m, err = s.Metrics(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Equal(t, 5.0, *m.Acceptance.MeanSeconds)
	assert.Equal(t, 10.0, *m.Blocked.MeanSeconds)
	assert.Zero(t, m.PendingAssignments)
}
