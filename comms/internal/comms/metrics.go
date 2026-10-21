package comms

import (
	"context"
	"database/sql"
	"time"
)

type Timing struct {
	Samples      int      `json:"samples"`
	TotalSeconds float64  `json:"total_seconds"`
	MeanSeconds  *float64 `json:"mean_seconds"`
}

func (t *Timing) add(start, end string) {
	a, e1 := time.Parse(time.RFC3339Nano, start)
	b, e2 := time.Parse(time.RFC3339Nano, end)
	if e1 != nil || e2 != nil || b.Before(a) {
		return
	}
	t.Samples++
	t.TotalSeconds += b.Sub(a).Seconds()
	mean := t.TotalSeconds / float64(t.Samples)
	t.MeanSeconds = &mean
}

type CoordinationMetrics struct {
	Acceptance                 Timing `json:"assignment_acceptance"`
	Blocked                    Timing `json:"closed_blocked_intervals"`
	Notification               Timing `json:"first_notification"`
	PendingAssignments         int    `json:"pending_assignments"`
	ReviewHandoffs             int    `json:"review_handoffs"`
	FailureEvents              int    `json:"failure_events"`
	RecordedDecisions          int    `json:"recorded_decisions"`
	DeclaredHumanInterventions int    `json:"declared_human_interventions"`
}

func (s *Store) Metrics(ctx context.Context, in TaskInput) (CoordinationMetrics, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return CoordinationMetrics{}, err
	}
	defer tx.Rollback()
	out, err := coordinationMetrics(ctx, tx, in.TaskID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func coordinationMetrics(ctx context.Context, db databaseReader, id string) (CoordinationMetrics, error) {
	var out CoordinationMetrics
	if _, err := scanTask(db.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", id)); err != nil {
		return out, err
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM assignments WHERE task_id=? AND state='offered'", id).Scan(&out.PendingAssignments); err != nil {
		return out, err
	}
	created := map[string]string{}
	accepted := map[string]bool{}
	blocked := map[string]string{}
	notified := map[string]string{}
	rows, err := db.QueryContext(ctx, "SELECT "+eventColumns+" FROM events WHERE task_id=? ORDER BY sequence", id)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		assignment, _ := e.Data["assignment_id"].(string)
		state, _ := e.Data["state"].(string)
		switch e.Kind {
		case "assignment_created":
			created[assignment] = e.CreatedAt
		case "assignment_updated", "assignment_submitted":
			if state == "accepted" && !accepted[assignment] {
				out.Acceptance.add(created[assignment], e.CreatedAt)
				accepted[assignment] = true
			}
			if start := blocked[assignment]; start != "" && !oneOf(state, "blocked", "needs_input") {
				out.Blocked.add(start, e.CreatedAt)
				delete(blocked, assignment)
			}
			if oneOf(state, "blocked", "needs_input") && blocked[assignment] == "" {
				blocked[assignment] = e.CreatedAt
			}
			if e.Kind == "assignment_submitted" {
				out.ReviewHandoffs++
			}
		case "delivery_submitted":
			message, _ := e.Data["message_id"].(string)
			if notified[message] == "" {
				notified[message] = e.CreatedAt
			}
		case "failure", "delivery_failed", "delivery_uncertain":
			out.FailureEvents++
		case "decision":
			out.RecordedDecisions++
		}
		if e.Data["event_type"] == "human_intervention" {
			out.DeclaredHumanInterventions++
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = db.QueryContext(ctx, "SELECT id,created_at FROM messages WHERE task_id=?", id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var message, at string
		if err := rows.Scan(&message, &at); err != nil {
			return out, err
		}
		if notified[message] != "" {
			out.Notification.add(at, notified[message])
		}
	}
	return out, rows.Err()
}
