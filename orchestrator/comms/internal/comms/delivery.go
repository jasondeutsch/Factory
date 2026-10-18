package comms

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Delivery records host notification only, never message acknowledgement or work.
type Delivery struct {
	TaskID        string `json:"task_id"`
	MessageID     string `json:"message_id"`
	ParticipantID string `json:"participant_id"`
	State         string `json:"state"`
	Attempts      int    `json:"attempts"`
	Detail        string `json:"detail"`
	UpdatedAt     string `json:"updated_at"`
	RetryAt       int64  `json:"retry_at"`
	LeaseToken    string `json:"lease_token,omitempty"`
}
type ClaimDeliveryInput struct {
	TaskID        string `json:"task_id"`
	MessageID     string `json:"message_id"`
	ParticipantID string `json:"participant_id"`
}
type DeliveryClaim struct {
	Delivery *Delivery `json:"delivery"`
	State    string    `json:"state"`
	Detail   string    `json:"detail,omitempty"`
}
type FinishDeliveryInput struct {
	ClaimDeliveryInput
	LeaseToken string `json:"lease_token"`
	State      string `json:"state" jsonschema:"submitted, retry, failed, or uncertain; terminal input uncertainty must never be automatically retried"`
	Detail     string `json:"detail"`
}
type RetryDeliveryInput struct {
	ClaimDeliveryInput
	Actor      string `json:"actor_participant_id"`
	RequestKey string `json:"request_key"`
}

const deliveryColumns = "task_id,message_id,participant_id,state,attempts,detail,updated_at,retry_at,lease_token"

func scanDelivery(row scanner) (Delivery, error) {
	var d Delivery
	err := row.Scan(&d.TaskID, &d.MessageID, &d.ParticipantID, &d.State, &d.Attempts, &d.Detail, &d.UpdatedAt, &d.RetryAt, &d.LeaseToken)
	return d, err
}

func (s *Store) ClaimDelivery(ctx context.Context, in ClaimDeliveryInput) (DeliveryClaim, error) {
	return s.claimDeliveryAt(ctx, in, time.Now())
}
func (s *Store) claimDeliveryAt(ctx context.Context, in ClaimDeliveryInput, at time.Time) (DeliveryClaim, error) {
	var out DeliveryClaim
	if err := required("task_id", in.TaskID, "message_id", in.MessageID, "participant_id", in.ParticipantID); err != nil {
		return out, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	participant, err := activeParticipant(ctx, tx, in.TaskID, in.ParticipantID)
	if err != nil {
		return out, err
	}
	message, err := scanMessage(tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE task_id=? AND id=?", in.TaskID, in.MessageID))
	if err != nil {
		return out, err
	}
	if message.Recipient != participant.RoutingIdentity {
		return out, errors.New("message is addressed to a different participant inbox")
	}
	if message.AcknowledgedAt != nil {
		out.State = "acknowledged"
		return out, nil
	}
	// Do not deliver old assignment offers to a replacement sharing its inbox.
	if message.Kind == "assignment" && message.AssignmentID != "" {
		assignment, err := assignmentRecord(ctx, tx, in.TaskID, message.AssignmentID)
		if err != nil {
			return out, err
		}
		if assignment.Assignee != participant.ID || oneOf(assignment.State, "submitted", "declined", "cancelled") {
			out.State = "ineligible"
			return out, nil
		}
	}
	// An expired lease may already have submitted terminal input. Never resend it
	// automatically. This is deliberately conservative across bridge crashes.
	rows, err := tx.QueryContext(ctx, "SELECT "+deliveryColumns+" FROM deliveries WHERE task_id=? AND participant_id=? AND state='claimed' AND retry_at<=?", in.TaskID, in.ParticipantID, at.Unix())
	if err != nil {
		return out, err
	}
	expired := []Delivery{}
	for rows.Next() {
		d, e := scanDelivery(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		expired = append(expired, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, d := range expired {
		d.State = "uncertain"
		d.Detail = "Bridge lease expired; terminal input may have been submitted. Inspect the session before retrying."
		d.UpdatedAt = at.UTC().Format(time.RFC3339Nano)
		d.LeaseToken = ""
		if err := saveDelivery(ctx, tx, d); err != nil {
			return out, err
		}
		if err := deliveryFailure(ctx, tx, d); err != nil {
			return out, err
		}
	}
	var active int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM deliveries WHERE task_id=? AND participant_id=? AND state='claimed'", in.TaskID, in.ParticipantID).Scan(&active); err != nil {
		return out, err
	}
	if active > 0 {
		out.State = "claimed"
		out.Detail = "Another delivery claim for this participant is still active"
		return out, tx.Commit()
	}
	d, err := scanDelivery(tx.QueryRowContext(ctx, "SELECT "+deliveryColumns+" FROM deliveries WHERE task_id=? AND message_id=? AND participant_id=?", in.TaskID, in.MessageID, in.ParticipantID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if err == nil && (d.State != "retry" || d.RetryAt > at.Unix()) {
		out.State, out.Detail = d.State, d.Detail
		return out, tx.Commit()
	}
	if errors.Is(err, sql.ErrNoRows) {
		d = Delivery{TaskID: in.TaskID, MessageID: in.MessageID, ParticipantID: in.ParticipantID}
	}
	d.State = "claimed"
	d.Attempts++
	d.LeaseToken = newID()
	d.RetryAt = at.Add(30 * time.Second).Unix()
	d.UpdatedAt = at.UTC().Format(time.RFC3339Nano)
	if err := saveDelivery(ctx, tx, d); err != nil {
		return out, err
	}
	out.Delivery = &d
	out.State = d.State
	return out, tx.Commit()
}
func saveDelivery(ctx context.Context, tx *sql.Tx, d Delivery) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO deliveries(task_id,message_id,participant_id,state,attempts,detail,updated_at,retry_at,lease_token) VALUES(?,?,?,?,?,?,?,?,?)
 ON CONFLICT(message_id,participant_id) DO UPDATE SET state=excluded.state,attempts=excluded.attempts,detail=excluded.detail,updated_at=excluded.updated_at,retry_at=excluded.retry_at,lease_token=excluded.lease_token`, d.TaskID, d.MessageID, d.ParticipantID, d.State, d.Attempts, d.Detail, d.UpdatedAt, d.RetryAt, d.LeaseToken)
	return err
}
func (s *Store) FinishDelivery(ctx context.Context, in FinishDeliveryInput) (Delivery, error) {
	if !oneOf(in.State, "submitted", "retry", "failed", "uncertain") {
		return Delivery{}, errors.New("invalid delivery state")
	}
	if err := required("lease_token", in.LeaseToken, "detail", in.Detail); err != nil {
		return Delivery{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Delivery{}, err
	}
	defer tx.Rollback()
	d, err := scanDelivery(tx.QueryRowContext(ctx, "SELECT "+deliveryColumns+" FROM deliveries WHERE task_id=? AND message_id=? AND participant_id=?", in.TaskID, in.MessageID, in.ParticipantID))
	if err != nil {
		return d, err
	}
	// Keep the completed token to make a lost finish response safe to repeat.
	if d.LeaseToken != in.LeaseToken {
		return d, errors.New("stale delivery lease")
	}
	if d.State != "claimed" {
		if (d.State == in.State || (in.State == "retry" && d.State == "failed" && d.Attempts >= 5)) && d.Detail == in.Detail {
			return d, nil
		}
		return d, errors.New("delivery lease already completed")
	}
	if d.RetryAt <= time.Now().Unix() {
		return d, errors.New("delivery lease expired; inspect delivery before retrying")
	}
	d.State = in.State
	d.Detail = in.Detail
	d.UpdatedAt = now()
	d.RetryAt = 0
	if d.State == "retry" {
		if d.Attempts >= 5 {
			d.State = "failed"
		} else {
			d.RetryAt = time.Now().Add(time.Duration(1<<d.Attempts) * 15 * time.Second).Unix()
		}
	}
	if err := saveDelivery(ctx, tx, d); err != nil {
		return d, err
	}
	if err := appendEvent(ctx, tx, d.TaskID, "delivery-bridge", "delivery_"+d.State, d.Detail, map[string]any{"message_id": d.MessageID, "participant_id": d.ParticipantID, "attempt": d.Attempts}); err != nil {
		return d, err
	}
	if oneOf(d.State, "retry", "failed", "uncertain") {
		if err := deliveryFailure(ctx, tx, d); err != nil {
			return d, err
		}
	}
	return d, tx.Commit()
}
func deliveryFailure(ctx context.Context, tx *sql.Tx, d Delivery) error {
	// One alert per message/participant, even across retries. Never recursively
	// alert the owner about a failed owner notification: its event is sufficient.
	task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", d.TaskID))
	if err != nil {
		return err
	}
	participant, err := participantRecord(ctx, tx, d.TaskID, d.ParticipantID)
	if err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, d.TaskID, "delivery-bridge", "delivery_attention", d.Detail, map[string]any{"message_id": d.MessageID, "participant_id": d.ParticipantID, "state": d.State}); err != nil {
		return err
	}
	if participant.RoutingIdentity == task.Owner {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"message_id": d.MessageID, "participant_id": d.ParticipantID, "state": d.State, "detail": d.Detail, "next_action": "Inspect list_deliveries and the target session. The original message remains unread; no assignment state changed."})
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO messages(id,task_id,sender,recipient,kind,text,created_at,request_key) VALUES(?,?,?,?,?,?,?,?)`, newID(), d.TaskID, "delivery-bridge", task.Owner, "finding", string(body), now(), "delivery-alert:"+d.MessageID+":"+d.ParticipantID)
	return err
}
func (s *Store) Deliveries(ctx context.Context, in TaskInput) ([]Delivery, error) {
	return deliveries(ctx, s.db, in.TaskID)
}
func deliveries(ctx context.Context, reader databaseReader, taskID string) ([]Delivery, error) {
	rows, err := reader.QueryContext(ctx, "SELECT "+deliveryColumns+" FROM deliveries WHERE task_id=? ORDER BY updated_at DESC,message_id DESC LIMIT 200", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		d.LeaseToken = ""
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) RetryDelivery(ctx context.Context, in RetryDeliveryInput) (Delivery, error) {
	return coordinationMutation(ctx, s, in.TaskID, in.Actor, in.RequestKey, "retry_delivery", in, func(tx *sql.Tx) (Delivery, error) {
		actor, err := activeParticipant(ctx, tx, in.TaskID, in.Actor)
		if err != nil {
			return Delivery{}, err
		}
		task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", in.TaskID))
		if err != nil {
			return Delivery{}, err
		}
		if actor.RoutingIdentity != task.Owner {
			return Delivery{}, errors.New("only the declared task owner can retry delivery")
		}
		if _, err := activeParticipant(ctx, tx, in.TaskID, in.ParticipantID); err != nil {
			return Delivery{}, err
		}
		d, err := scanDelivery(tx.QueryRowContext(ctx, "SELECT "+deliveryColumns+" FROM deliveries WHERE task_id=? AND message_id=? AND participant_id=?", in.TaskID, in.MessageID, in.ParticipantID))
		if err != nil {
			return d, err
		}
		if !oneOf(d.State, "failed", "uncertain", "submitted") {
			return d, fmt.Errorf("cannot manually retry %s delivery", d.State)
		}
		d.State = "retry"
		d.RetryAt = 0
		d.Attempts = 0
		d.LeaseToken = ""
		d.UpdatedAt = now()
		d.Detail = "Explicit retry authorized by declared task owner after session inspection"
		if err := saveDelivery(ctx, tx, d); err != nil {
			return d, err
		}
		if err := appendEvent(ctx, tx, d.TaskID, actor.RoutingIdentity, "delivery_retry", d.Detail, map[string]any{"message_id": d.MessageID, "participant_id": d.ParticipantID}); err != nil {
			return d, err
		}
		return d, nil
	})
}
