package comms

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStatusUsesConfiguredMembersAndActualInstances(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "status")
	input := SetTaskFactoryInput{TaskID: task.ID, Factory: FactorySnapshot{Name: "custom", Owner: "coordinator", Agents: []FactoryMember{{Name: "coordinator", Path: "../../orchestrator"}, {Name: "writer", Path: "agents/writer"}}}}
	_, err := s.SetTaskFactory(t.Context(), input)
	require.NoError(t, err)
	_, err = s.SetTaskFactory(t.Context(), input)
	require.NoError(t, err)
	testParticipant(t, s, task, "coordinator")
	view, err := s.Status(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	require.Len(t, view.Members, 2)
	assert.Empty(t, view.Members[1].Instances)
	var html bytes.Buffer
	require.NoError(t, RenderStatus(&html, view))
	assert.Contains(t, html.String(), "Not started")
	assert.Contains(t, html.String(), "custom")
	input.Factory.Name = "changed"
	_, err = s.SetTaskFactory(t.Context(), input)
	require.ErrorContains(t, err, "immutable")
}
