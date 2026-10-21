package comms

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Attention struct {
	ID          string   `json:"id"`
	TaskID      string   `json:"task_id"`
	Kind        string   `json:"kind"`
	Recipient   string   `json:"recipient"`
	Title       string   `json:"title"`
	Detail      string   `json:"detail"`
	Options     []string `json:"options"`
	References  []string `json:"references"`
	State       string   `json:"state"`
	Version     int64    `json:"version"`
	Resolution  string   `json:"resolution"`
	Explanation string   `json:"explanation"`
	CreatedAt   string   `json:"created_at"`
	ResolvedAt  string   `json:"resolved_at"`
}
type RequestAttentionInput struct {
	TaskID     string   `json:"task_id"`
	Actor      string   `json:"actor_participant_id"`
	Recipient  string   `json:"recipient"`
	Kind       string   `json:"kind" jsonschema:"question, approval, review, failure, or blocker"`
	Title      string   `json:"title"`
	Detail     string   `json:"detail"`
	Options    []string `json:"options,omitempty"`
	References []string `json:"references,omitempty"`
	RequestKey string   `json:"request_key"`
}
type ResolveAttentionInput struct {
	TaskID          string `json:"task_id"`
	ID              string `json:"attention_id"`
	Actor           string `json:"actor_participant_id"`
	ExpectedVersion int64  `json:"expected_version"`
	Resolution      string `json:"resolution"`
	Explanation     string `json:"explanation"`
	RequestKey      string `json:"request_key"`
}
type CoordinatorInbox struct {
	Items            []Attention `json:"items"`
	OpenCount        int         `json:"open_count"`
	DeliveryFailures []Delivery  `json:"delivery_failures"`
	Partial          bool        `json:"partial"`
}

const attentionColumns = "id,task_id,kind,recipient,title,detail,options,refs,state,version,resolution,explanation,created_at,resolved_at"

func scanAttention(row scanner) (Attention, error) {
	var a Attention
	var options, refs string
	err := row.Scan(&a.ID, &a.TaskID, &a.Kind, &a.Recipient, &a.Title, &a.Detail, &options, &refs, &a.State, &a.Version, &a.Resolution, &a.Explanation, &a.CreatedAt, &a.ResolvedAt)
	if err != nil {
		return a, err
	}
	if err = json.Unmarshal([]byte(options), &a.Options); err != nil {
		return a, err
	}
	err = json.Unmarshal([]byte(refs), &a.References)
	return a, err
}
func createAttention(ctx context.Context, tx *sql.Tx, in RequestAttentionInput) (Attention, error) {
	options, _ := json.Marshal(in.Options)
	refs, _ := json.Marshal(in.References)
	id := newID()
	_, err := tx.ExecContext(ctx, "INSERT INTO attention("+attentionColumns+") VALUES(?,?,?,?,?,?,?,?,'open',1,'','',?,'')", id, in.TaskID, in.Kind, in.Recipient, in.Title, in.Detail, string(options), string(refs), now())
	if err != nil {
		return Attention{}, err
	}
	a, err := scanAttention(tx.QueryRowContext(ctx, "SELECT "+attentionColumns+" FROM attention WHERE id=?", id))
	if err != nil {
		return a, err
	}
	err = appendEvent(ctx, tx, in.TaskID, in.Actor, "attention_requested", in.Title, map[string]any{"attention_id": id, "kind": in.Kind, "recipient": in.Recipient, "references": in.References})
	return a, err
}
func (s *Store) RequestAttention(ctx context.Context, in RequestAttentionInput) (Attention, error) {
	if err := required("recipient", in.Recipient, "title", in.Title, "detail", in.Detail); err != nil {
		return Attention{}, err
	}
	if !oneOf(in.Kind, "question", "approval", "review", "failure", "blocker") {
		return Attention{}, errors.New("invalid attention kind")
	}
	for _, option := range in.Options {
		if err := required("option", option); err != nil {
			return Attention{}, err
		}
	}
	return coordinationMutation(ctx, s, in.TaskID, "participant:"+in.Actor, in.RequestKey, "request_attention", in, func(tx *sql.Tx) (Attention, error) {
		if _, err := activeParticipant(ctx, tx, in.TaskID, in.Actor); err != nil {
			return Attention{}, err
		}
		return createAttention(ctx, tx, in)
	})
}
func (s *Store) ResolveAttention(ctx context.Context, in ResolveAttentionInput) (Attention, error) {
	if err := required("resolution", in.Resolution, "explanation", in.Explanation); err != nil {
		return Attention{}, err
	}
	return coordinationMutation(ctx, s, in.TaskID, "participant:"+in.Actor, in.RequestKey, "resolve_attention", in, func(tx *sql.Tx) (Attention, error) {
		a, err := scanAttention(tx.QueryRowContext(ctx, "SELECT "+attentionColumns+" FROM attention WHERE task_id=? AND id=?", in.TaskID, in.ID))
		if err != nil {
			return a, err
		}
		actor, err := activeParticipant(ctx, tx, in.TaskID, in.Actor)
		if err != nil {
			return a, err
		}
		task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", in.TaskID))
		if err != nil {
			return a, err
		}
		if actor.RoutingIdentity != task.Owner {
			return a, errors.New("only the task owner records the actual resolution; human authority remains external")
		}
		if a.State != "open" || a.Version != in.ExpectedVersion {
			return a, errors.New("stale or resolved attention item")
		}
		if len(a.Options) > 0 && !oneOf(in.Resolution, a.Options...) {
			return a, errors.New("resolution must select one of the requested options")
		}
		_, err = tx.ExecContext(ctx, "UPDATE attention SET state='resolved',version=version+1,resolution=?,explanation=?,resolved_at=? WHERE id=?", in.Resolution, in.Explanation, now(), a.ID)
		if err != nil {
			return a, err
		}
		err = appendEvent(ctx, tx, in.TaskID, actor.RoutingIdentity, "attention_resolved", in.Explanation, map[string]any{"attention_id": a.ID, "resolution": in.Resolution, "references": a.References})
		if err != nil {
			return a, err
		}
		return scanAttention(tx.QueryRowContext(ctx, "SELECT "+attentionColumns+" FROM attention WHERE id=?", a.ID))
	})
}
func (s *Store) InboxSummary(ctx context.Context, in TaskInput) (CoordinatorInbox, error) {
	out := CoordinatorInbox{Items: []Attention{}, DeliveryFailures: []Delivery{}}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if _, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", in.TaskID)); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM attention WHERE task_id=? AND state='open'", in.TaskID).Scan(&out.OpenCount); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+attentionColumns+" FROM attention WHERE task_id=? AND state='open' ORDER BY id LIMIT 200", in.TaskID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		a, e := scanAttention(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	items, err := deliveries(ctx, tx, in.TaskID)
	if err != nil {
		return out, err
	}
	for _, d := range items {
		if oneOf(d.State, "failed", "uncertain", "retry") {
			out.DeliveryFailures = append(out.DeliveryFailures, d)
		}
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM deliveries WHERE task_id=? AND state IN ('failed','uncertain','retry')", in.TaskID).Scan(&count); err != nil {
		return out, err
	}
	out.Partial = out.OpenCount > len(out.Items) || count > len(out.DeliveryFailures)
	return out, tx.Commit()
}
