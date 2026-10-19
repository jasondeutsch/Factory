package comms

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// SessionManifest is an immutable description of a prepared runtime snapshot.
// Never add raw configuration, environment values or credential fields here.
type SkillSnapshot struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type SessionManifest struct {
	Delivery                 string          `json:"delivery,omitempty"`
	Runner                   string          `json:"runner"`
	Provider                 string          `json:"provider,omitempty"`
	Model                    string          `json:"model,omitempty"`
	Thinking                 string          `json:"thinking,omitempty"`
	Skills                   []SkillSnapshot `json:"skills"`
	MCPs                     []string        `json:"mcps"`
	ConfigurationFingerprint string          `json:"configuration_fingerprint"`
	PreparedAt               string          `json:"prepared_at"`
	StartupChecked           bool            `json:"startup_checked"`
}
type SessionInput struct {
	TaskID        string `json:"task_id"`
	ParticipantID string `json:"participant_id"`
}
type RecordSessionInput struct {
	TaskID        string          `json:"task_id"`
	ParticipantID string          `json:"participant_id"`
	Manifest      SessionManifest `json:"manifest"`
	RequestKey    string          `json:"request_key"`
}
type ContactInput struct {
	TaskID             string `json:"task_id"`
	ParticipantID      string `json:"participant_id"`
	ExpectedSessionRef string `json:"expected_session_ref"`
	State              string `json:"state" jsonschema:"idle, working, blocked, unknown, or unavailable; host observation not assignment progress"`
}
type SessionSnapshot struct {
	ParticipantID string          `json:"participant_id"`
	Manifest      SessionManifest `json:"manifest"`
	ContactState  string          `json:"contact_state"`
	ObservedAt    string          `json:"observed_at,omitempty"`
	ExpiresAt     string          `json:"expires_at,omitempty"`
}

var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *Store) RecordSession(ctx context.Context, in RecordSessionInput) (SessionSnapshot, error) {
	if !oneOf(in.Manifest.Delivery, "", "native", "manual") {
		return SessionSnapshot{}, errors.New("delivery must be native or manual")
	}
	if in.Manifest.Runner == "" || !digest.MatchString(in.Manifest.ConfigurationFingerprint) {
		return SessionSnapshot{}, errors.New("runner and SHA256 configuration fingerprint are required")
	}
	if _, err := time.Parse(time.RFC3339Nano, in.Manifest.PreparedAt); err != nil {
		return SessionSnapshot{}, errors.New("prepared_at must be an RFC3339 timestamp")
	}
	for _, skill := range in.Manifest.Skills {
		if skill.Path == "" || !digest.MatchString(skill.SHA256) {
			return SessionSnapshot{}, errors.New("skills need a path and SHA256")
		}
	}
	return coordinationMutation(ctx, s, in.TaskID, in.ParticipantID, in.RequestKey, "record_session", in, func(tx *sql.Tx) (SessionSnapshot, error) {
		if _, err := activeParticipant(ctx, tx, in.TaskID, in.ParticipantID); err != nil {
			return SessionSnapshot{}, err
		}
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sessions WHERE participant_id=?", in.ParticipantID).Scan(&exists); err != nil {
			return SessionSnapshot{}, err
		}
		if exists != 0 {
			return SessionSnapshot{}, errors.New("session already recorded; snapshots are immutable, reuse the original request key")
		}
		encoded, _ := json.Marshal(in.Manifest)
		_, err := tx.ExecContext(ctx, "INSERT INTO sessions(participant_id,manifest) VALUES(?,?)", in.ParticipantID, string(encoded))
		if err != nil {
			return SessionSnapshot{}, fmt.Errorf("record session snapshot: %w", err)
		}
		return SessionSnapshot{ParticipantID: in.ParticipantID, Manifest: in.Manifest, ContactState: "unknown"}, nil
	})
}
func (s *Store) Session(ctx context.Context, in SessionInput) (SessionSnapshot, error) {
	return sessionSnapshot(ctx, s.db, in, time.Now().UTC())
}
func sessionSnapshot(ctx context.Context, db databaseReader, in SessionInput, at time.Time) (SessionSnapshot, error) {
	p, err := participantRecord(ctx, db, in.TaskID, in.ParticipantID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	out := SessionSnapshot{ParticipantID: p.ID}
	var encoded string
	err = db.QueryRowContext(ctx, "SELECT manifest,contact_state,observed_at,expires_at FROM sessions WHERE participant_id=?", p.ID).Scan(&encoded, &out.ContactState, &out.ObservedAt, &out.ExpiresAt)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(encoded), &out.Manifest); err != nil {
		return out, err
	}
	expiry, _ := time.Parse(time.RFC3339Nano, out.ExpiresAt)
	if p.RetiredAt != nil || !at.Before(expiry) {
		out.ContactState = "unknown"
	}
	return out, nil
}
func (s *Store) RecordContact(ctx context.Context, in ContactInput) (SessionSnapshot, error) {
	switch in.State {
	case "idle", "working", "blocked", "unknown", "unavailable":
	default:
		return SessionSnapshot{}, errors.New("invalid contact state")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionSnapshot{}, err
	}
	defer tx.Rollback()
	p, err := activeParticipant(ctx, tx, in.TaskID, in.ParticipantID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	if p.SessionRef == "" || p.SessionRef != in.ExpectedSessionRef {
		return SessionSnapshot{}, errors.New("contact requires the current captured session binding")
	}
	at := time.Now().UTC()
	result, err := tx.ExecContext(ctx, "UPDATE sessions SET contact_state=?,observed_at=?,expires_at=? WHERE participant_id=?", in.State, at.Format(time.RFC3339Nano), at.Add(30*time.Second).Format(time.RFC3339Nano), p.ID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return SessionSnapshot{}, errors.New("session manifest not recorded")
	}
	out, err := sessionSnapshot(ctx, tx, SessionInput{in.TaskID, p.ID}, at)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
