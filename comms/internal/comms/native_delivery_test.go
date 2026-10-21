package comms

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMessageSentReadExec(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "project")
	ctx := t.Context()
	m, err := s.SendMessage(ctx, SendMessageInput{TaskID: task.ID, Sender: "coordinator", Recipient: "coder", Kind: "question", Text: "Inspect this"})
	require.NoError(t, err)
	require.Equal(t, "sent", m.Status)
	input := AcknowledgeInput{TaskID: task.ID, MessageID: m.ID, Actor: "coder"}
	_, err = s.ExecuteMessage(ctx, input)
	require.ErrorContains(t, err, "read")
	_, err = s.Acknowledge(ctx, AcknowledgeInput{TaskID: task.ID, MessageID: m.ID, Actor: "other"})
	require.Error(t, err)
	m, err = s.Acknowledge(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "read", m.Status)
	m, err = s.ExecuteMessage(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "exec", m.Status)
	require.NotNil(t, m.ExecutedAt)
	again, err := s.ExecuteMessage(ctx, input)
	require.NoError(t, err)
	require.Equal(t, m.ExecutedAt, again.ExecutedAt)
	again, err = s.Acknowledge(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "exec", again.Status)
}
func TestNativeContactRequiresExactActiveInstance(t *testing.T) {
	s := testStore(t)
	task := testTask(t, s, "project")
	ctx := t.Context()
	p, err := s.RegisterParticipant(ctx, RegisterParticipantInput{TaskID: task.ID, InstanceID: "actual", Role: "coder", RoutingIdentity: "coder", RequestKey: "p"})
	require.NoError(t, err)
	_, err = s.RecordSession(ctx, RecordSessionInput{TaskID: task.ID, ParticipantID: p.ID, Manifest: SessionManifest{Runner: "pi", Delivery: "native", ConfigurationFingerprint: strings.Repeat("a", 64), PreparedAt: time.Now().UTC().Format(time.RFC3339Nano)}, RequestKey: "s"})
	require.NoError(t, err)
	_, err = s.RecordRunnerContact(ctx, RunnerContactInput{TaskID: task.ID, ParticipantID: p.ID, InstanceID: "wrong", State: "idle"})
	require.Error(t, err)
	contact, err := s.RecordRunnerContact(ctx, RunnerContactInput{TaskID: task.ID, ParticipantID: p.ID, InstanceID: "actual", State: "idle"})
	require.NoError(t, err)
	require.Equal(t, "idle", contact.ContactState)
	_, err = s.RegisterParticipant(ctx, RegisterParticipantInput{TaskID: task.ID, InstanceID: "replacement", Role: "coder", RoutingIdentity: "coder", ReplacesParticipantID: p.ID, RequestKey: "replace"})
	require.NoError(t, err)
	_, err = s.RecordRunnerContact(ctx, RunnerContactInput{TaskID: task.ID, ParticipantID: p.ID, InstanceID: "actual", State: "idle"})
	require.Error(t, err)
}
