package comms

import (
	"context"
	"errors"
	"time"
)

// Exec is an explicit declaration by the recipient, never a transport inference.
func (s *Store) ExecuteMessage(ctx context.Context, in AcknowledgeInput) (Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()
	message, err := scanMessage(tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE task_id=? AND id=?", in.TaskID, in.MessageID))
	if err != nil {
		return Message{}, err
	}
	if message.Recipient != in.Actor {
		return Message{}, errors.New("only the declared recipient can execute this message")
	}
	if message.AcknowledgedAt == nil {
		return Message{}, errors.New("read the message before declaring exec")
	}
	if message.ExecutedAt != nil {
		return message, nil
	}
	at := now()
	if _, err := tx.ExecContext(ctx, "UPDATE messages SET executed_at=? WHERE id=?", at, message.ID); err != nil {
		return Message{}, err
	}
	if err := appendEvent(ctx, tx, in.TaskID, in.Actor, "message_exec", "Recipient began acting on message", map[string]string{"message_id": message.ID}); err != nil {
		return Message{}, err
	}
	message.ExecutedAt = &at
	message.Status = "exec"
	return message, tx.Commit()
}

type RunnerContactInput struct {
	TaskID        string `json:"task_id"`
	ParticipantID string `json:"participant_id"`
	InstanceID    string `json:"instance_id"`
	State         string `json:"state"`
}

func (s *Store) RecordRunnerContact(ctx context.Context, in RunnerContactInput) (SessionSnapshot, error) {
	if !oneOf(in.State, "idle", "working", "blocked", "unavailable") {
		return SessionSnapshot{}, errors.New("invalid runner contact state")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionSnapshot{}, err
	}
	defer tx.Rollback()
	participant, err := activeParticipant(ctx, tx, in.TaskID, in.ParticipantID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	if in.InstanceID == "" || participant.InstanceID != in.InstanceID {
		return SessionSnapshot{}, errors.New("runner instance does not match active participant")
	}
	snapshot, err := sessionSnapshot(ctx, tx, SessionInput{in.TaskID, in.ParticipantID}, time.Now().UTC())
	if err != nil {
		return snapshot, err
	}
	if snapshot.Manifest.Delivery != "native" {
		return snapshot, errors.New("session did not declare native delivery")
	}
	at := time.Now().UTC()
	_, err = tx.ExecContext(ctx, "UPDATE sessions SET contact_state=?,observed_at=?,expires_at=? WHERE participant_id=?", in.State, at.Format(time.RFC3339Nano), at.Add(30*time.Second).Format(time.RFC3339Nano), in.ParticipantID)
	if err != nil {
		return snapshot, err
	}
	snapshot, err = sessionSnapshot(ctx, tx, SessionInput{in.TaskID, in.ParticipantID}, at)
	if err != nil {
		return snapshot, err
	}
	return snapshot, tx.Commit()
}
