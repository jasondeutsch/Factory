package comms

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitorTasksFiltersAndPaginatesByProject(t *testing.T) {
	store := testStore(t)
	older := testTask(t, store, "team-a")
	newer := testTask(t, store, "team-a")
	testTask(t, store, "team-b")

	projects, err := store.Projects(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"team-a", "team-b"}, projects)

	page, err := store.MonitorTasks(t.Context(), MonitorTasksInput{Project: "team-a", Limit: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, newer.ID, page[0].Task.ID)

	page, err = store.MonitorTasks(t.Context(), MonitorTasksInput{Project: "team-a", Before: page[0].Task.Sequence})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, older.ID, page[0].Task.ID)
}
