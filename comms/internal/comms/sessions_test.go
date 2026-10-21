package comms

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestSessionSnapshotContactAndExpiry(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "session-project")
	worker := testParticipant(t, s, task, "worker")
	input := RecordSessionInput{TaskID: task.ID, ParticipantID: worker.ID, RequestKey: "snapshot", Manifest: SessionManifest{
		Runner: "pi", Provider: "fixture", Model: "test", PreparedAt: now(), StartupChecked: true,
		ConfigurationFingerprint: strings.Repeat("a", 64), Skills: []SkillSnapshot{{Path: "role.md", SHA256: strings.Repeat("b", 64)}}, MCPs: []string{"comms"},
	}}
	snapshot, err := s.RecordSession(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, "unknown", snapshot.ContactState)
	replay, err := s.RecordSession(t.Context(), input)
	require.NoError(t, err)
	assert.Equal(t, snapshot, replay)
	input.Manifest.Model = "changed"
	_, err = s.RecordSession(t.Context(), input)
	require.Error(t, err)
	input.RequestKey = "overwrite"
	_, err = s.RecordSession(t.Context(), input)
	require.ErrorContains(t, err, "immutable")
	other := testTask(t, s, "other")
	_, err = s.Session(t.Context(), SessionInput{other.ID, worker.ID})
	require.Error(t, err)
	_, err = s.RecordContact(t.Context(), ContactInput{TaskID: task.ID, ParticipantID: worker.ID, State: "idle"})
	require.ErrorContains(t, err, "binding")
	_, err = s.BindParticipant(t.Context(), BindParticipantInput{TaskID: task.ID, ParticipantID: worker.ID, Actor: worker.ID, SessionRef: "captured", RequestKey: "bind"})
	require.NoError(t, err)
	snapshot, err = s.RecordContact(t.Context(), ContactInput{TaskID: task.ID, ParticipantID: worker.ID, ExpectedSessionRef: "captured", State: "blocked"})
	require.NoError(t, err)
	assert.Equal(t, "blocked", snapshot.ContactState)
	assert.Equal(t, "test", snapshot.Manifest.Model)
	expired, err := sessionSnapshot(t.Context(), s.db, SessionInput{task.ID, worker.ID}, time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, "unknown", expired.ContactState)
	report, err := s.Report(t.Context(), task.ID)
	require.NoError(t, err)
	require.Len(t, report.Sessions, 1)
	assert.Equal(t, "blocked", report.Sessions[0].ContactState)
	_, err = s.BindParticipant(t.Context(), BindParticipantInput{TaskID: task.ID, ParticipantID: worker.ID, Actor: worker.ID, ExpectedSessionRef: "captured", SessionRef: "changed", RequestKey: "rebind"})
	require.NoError(t, err)
	changed, err := s.Session(t.Context(), SessionInput{task.ID, worker.ID})
	require.NoError(t, err)
	assert.Equal(t, "unknown", changed.ContactState)
	assert.Empty(t, changed.ObservedAt)
	_, err = s.RecordContact(t.Context(), ContactInput{TaskID: task.ID, ParticipantID: worker.ID, ExpectedSessionRef: "captured", State: "idle"})
	require.ErrorContains(t, err, "binding")
	_, err = s.RegisterParticipant(t.Context(), RegisterParticipantInput{TaskID: task.ID, Role: worker.Role, RoutingIdentity: worker.RoutingIdentity, InstanceID: "new-worker", ReplacesParticipantID: worker.ID, RequestKey: "replace"})
	require.NoError(t, err)
	_, err = s.RecordContact(t.Context(), ContactInput{TaskID: task.ID, ParticipantID: worker.ID, ExpectedSessionRef: "captured", State: "working"})
	require.ErrorContains(t, err, "retired")
	retired, err := s.Session(t.Context(), SessionInput{task.ID, worker.ID})
	require.NoError(t, err)
	assert.Equal(t, "unknown", retired.ContactState)
}
