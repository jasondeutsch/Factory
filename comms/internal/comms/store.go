package comms

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct{ db *sql.DB }

var errTaskNotFound = errors.New("task not found")
var errMessageNotFound = errors.New("message not found for this task")

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path must not be empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute}).String() + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection serializes short transactions without deferred-transaction
	// upgrade races. The service owns the database; clients never open it directly.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	failed := func(err error) (*Store, error) { db.Close(); return nil, err }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;"); err != nil {
		return failed(err)
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return failed(err)
	}
	if version > 2 {
		return failed(fmt.Errorf("database schema %d is newer than supported schema 2", version))
	}
	if err := initializeSchema(ctx, db, version); err != nil {
		return failed(err)
	}
	return &Store{db: db}, nil
}

func initializeSchema(ctx context.Context, db *sql.DB, version int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	if version < 2 {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE messages ADD COLUMN assignment_id TEXT REFERENCES assignments(id)"); err != nil {
			return fmt.Errorf("upgrade messages: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version=2"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func now() string                               { return time.Now().UTC().Format(time.RFC3339Nano) }
func newID() string                             { return time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + rand.Text() }

func required(fields ...string) error {
	for i := 0; i < len(fields); i += 2 {
		if strings.TrimSpace(fields[i+1]) == "" {
			return fmt.Errorf("%s must not be empty", fields[i])
		}
		if len(fields[i+1]) > 1<<20 {
			return fmt.Errorf("%s exceeds 1 MiB", fields[i])
		}
	}
	return nil
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func pageLimit(value, fallback, maximum int) (int, error) {
	if value == 0 {
		return fallback, nil
	}
	if value < 1 || value > maximum {
		return 0, fmt.Errorf("limit must be between 1 and %d", maximum)
	}
	return value, nil
}

const taskColumns = "sequence,id,project,name,brief,owner,status,summary,version,created_at,updated_at"
const messageColumns = "id,task_id,sender,recipient,kind,text,created_at,acknowledged_at,assignment_id"
const eventColumns = "sequence,task_id,actor,kind,stage,attempt,message,data,created_at"

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (Task, error) {
	var task Task
	err := row.Scan(&task.Sequence, &task.ID, &task.Project, &task.Name, &task.Brief, &task.Owner, &task.Status, &task.Summary, &task.Version, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = errTaskNotFound
	}
	return task, err
}
func scanMessage(row scanner) (Message, error) {
	var message Message
	var assignmentID sql.NullString
	err := row.Scan(&message.ID, &message.TaskID, &message.Sender, &message.Recipient, &message.Kind, &message.Text, &message.CreatedAt, &message.AcknowledgedAt, &assignmentID)
	message.AssignmentID = assignmentID.String
	if errors.Is(err, sql.ErrNoRows) {
		err = errMessageNotFound
	}
	return message, err
}
func scanEvent(row scanner) (Event, error) {
	var event Event
	var data string
	err := row.Scan(&event.Sequence, &event.TaskID, &event.Actor, &event.Kind, &event.Stage, &event.Attempt, &event.Message, &data, &event.CreatedAt)
	if err == nil {
		err = json.Unmarshal([]byte(data), &event.Data)
	}
	return event, err
}

func appendEvent(ctx context.Context, tx *sql.Tx, taskID, actor, kind, message string, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO events(task_id,actor,kind,message,data,created_at) VALUES(?,?,?,?,?,?)", taskID, actor, kind, message, string(encoded), now())
	return err
}

func (s *Store) CreateTask(ctx context.Context, input CreateTaskInput) (Task, error) {
	if err := required("project", input.Project, "name", input.Name, "brief", input.Brief, "owner", input.Owner); err != nil {
		return Task{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	if input.RequestKey != "" {
		old, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE project=? AND request_key=?", input.Project, input.RequestKey))
		if err == nil {
			if old.Name != input.Name || old.Brief != input.Brief || old.Owner != input.Owner {
				return Task{}, errors.New("request_key was already used for a different task")
			}
			return old, nil
		}
		if !errors.Is(err, errTaskNotFound) {
			return Task{}, err
		}
	}
	id, at := newID(), now()
	_, err = tx.ExecContext(ctx, "INSERT INTO tasks(id,project,name,brief,owner,status,summary,created_at,updated_at,request_key) VALUES(?,?,?,?,?,'pending','Not started',?,?,NULLIF(?,''))", id, input.Project, input.Name, input.Brief, input.Owner, at, at, input.RequestKey)
	if err != nil {
		return Task{}, err
	}
	if err := appendEvent(ctx, tx, id, input.Owner, "task_created", "Task created", map[string]any{"name": input.Name}); err != nil {
		return Task{}, err
	}
	task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", id))
	if err != nil {
		return Task{}, err
	}
	return task, tx.Commit()
}

func (s *Store) Task(ctx context.Context, id string) (Task, error) {
	return scanTask(s.db.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", id))
}

func (s *Store) Tasks(ctx context.Context, input ListTasksInput) ([]Task, error) {
	if err := required("project", input.Project); err != nil {
		return nil, err
	}
	limit, err := pageLimit(input.Limit, 20, 100)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE project=? AND (?=0 OR sequence<?) ORDER BY sequence DESC LIMIT ?", input.Project, input.Before, input.Before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) UpdateTask(ctx context.Context, input UpdateTaskInput) (Task, error) {
	if err := required("actor", input.Actor, "summary", input.Summary); err != nil {
		return Task{}, err
	}
	if !oneOf(input.Status, "pending", "working", "needs_input", "blocked", "complete", "cancelled") {
		return Task{}, errors.New("invalid task status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", input.TaskID))
	if err != nil {
		return Task{}, err
	}
	if input.Actor != task.Owner {
		return Task{}, errors.New("only the declared task owner can update status")
	}
	if input.ExpectedVersion != task.Version {
		return Task{}, errors.New("stale task version; read the current task before updating")
	}
	_, err = tx.ExecContext(ctx, "UPDATE tasks SET status=?,summary=?,version=version+1,updated_at=? WHERE id=?", input.Status, input.Summary, now(), input.TaskID)
	if err != nil {
		return Task{}, err
	}
	if err := appendEvent(ctx, tx, task.ID, input.Actor, "status_changed", input.Summary, map[string]any{"from": task.Status, "to": input.Status}); err != nil {
		return Task{}, err
	}
	task, err = scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", input.TaskID))
	if err != nil {
		return Task{}, err
	}
	return task, tx.Commit()
}

func (s *Store) SendMessage(ctx context.Context, input SendMessageInput) (Message, error) {
	if err := required("task_id", input.TaskID, "sender", input.Sender, "recipient", input.Recipient, "text", input.Text); err != nil {
		return Message{}, err
	}
	if !oneOf(input.Kind, "assignment", "question", "answer", "progress", "finding", "result", "decision") {
		return Message{}, errors.New("invalid message kind")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()
	if input.RequestKey != "" {
		message, err := scanMessage(tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE task_id=? AND sender=? AND request_key=?", input.TaskID, input.Sender, input.RequestKey))
		if err == nil {
			if message.Recipient != input.Recipient || message.Kind != input.Kind || message.Text != input.Text || message.AssignmentID != input.AssignmentID {
				return Message{}, errors.New("request_key was already used for a different message")
			}
			return message, nil
		}
		if !errors.Is(err, errMessageNotFound) {
			return Message{}, err
		}
	}
	if input.AssignmentID != "" {
		if _, err := assignmentRecord(ctx, tx, input.TaskID, input.AssignmentID); err != nil {
			return Message{}, err
		}
	}

	id := newID()
	_, err = tx.ExecContext(ctx, "INSERT INTO messages(id,task_id,sender,recipient,kind,text,created_at,request_key,assignment_id) VALUES(?,?,?,?,?,?,?,NULLIF(?,''),NULLIF(?,''))", id, input.TaskID, input.Sender, input.Recipient, input.Kind, input.Text, now(), input.RequestKey, input.AssignmentID)
	if err != nil {
		return Message{}, fmt.Errorf("save message (task must exist): %w", err)
	}
	if err := appendEvent(ctx, tx, input.TaskID, input.Sender, "message_sent", "Message saved", map[string]any{"message_id": id, "recipient": input.Recipient, "kind": input.Kind}); err != nil {
		return Message{}, err
	}
	message, err := scanMessage(tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id=?", id))
	if err != nil {
		return Message{}, err
	}
	return message, tx.Commit()
}

func (s *Store) Inbox(ctx context.Context, input InboxInput) ([]Message, error) {
	if err := required("task_id", input.TaskID, "recipient", input.Recipient); err != nil {
		return nil, err
	}
	if _, err := s.Task(ctx, input.TaskID); err != nil {
		return nil, err
	}
	limit, err := pageLimit(input.Limit, 50, 200)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE task_id=? AND recipient=? AND (? OR acknowledged_at IS NULL) AND id>? ORDER BY id LIMIT ?", input.TaskID, input.Recipient, input.IncludeAcknowledged, input.After, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []Message{}
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *Store) Acknowledge(ctx context.Context, input AcknowledgeInput) (Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()
	message, err := scanMessage(tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE task_id=? AND id=?", input.TaskID, input.MessageID))
	if err != nil {
		return Message{}, err
	}
	if input.Actor != message.Recipient {
		return Message{}, errors.New("only the declared recipient can acknowledge this message")
	}
	if message.AcknowledgedAt != nil {
		return message, nil
	}
	at := now()
	if _, err := tx.ExecContext(ctx, "UPDATE messages SET acknowledged_at=? WHERE id=?", at, message.ID); err != nil {
		return Message{}, err
	}
	if err := appendEvent(ctx, tx, message.TaskID, input.Actor, "message_acknowledged", "Message read", map[string]string{"message_id": message.ID}); err != nil {
		return Message{}, err
	}
	message.AcknowledgedAt = &at
	return message, tx.Commit()
}

func (s *Store) RecordEvent(ctx context.Context, input RecordEventInput) (Event, error) {
	if !oneOf(input.Kind, "progress", "check", "finding", "failure", "decision", "note", "stage_started", "stage_completed") {
		return Event{}, errors.New("invalid event kind")
	}
	if (input.Kind == "stage_started" || input.Kind == "stage_completed") && (strings.TrimSpace(input.Stage) == "" || input.Attempt < 1) {
		return Event{}, errors.New("stage events require stage and a positive attempt number")
	}
	return s.recordEvent(ctx, input)
}

func (s *Store) RecordUsage(ctx context.Context, input RecordUsageInput) (Event, error) {
	usage := input.Usage
	if err := required("stage", input.Stage, "request_key", input.RequestKey, "provider", usage.Provider, "model", usage.Model); err != nil {
		return Event{}, err
	}
	if input.Attempt < 1 || !oneOf(usage.Source, "runtime", "provider", "agent_report") {
		return Event{}, errors.New("usage requires a positive attempt and source runtime, provider, or agent_report")
	}
	for _, value := range []*int64{usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheWriteTokens} {
		if value != nil && (*value < 0 || *value > 1_000_000_000_000) {
			return Event{}, errors.New("token counts must be between zero and 1e12")
		}
	}
	if usage.CostUSD != nil && (*usage.CostUSD < 0 || math.IsNaN(*usage.CostUSD) || math.IsInf(*usage.CostUSD, 0)) {
		return Event{}, errors.New("cost_usd must be finite and nonnegative")
	}
	if usage.InputTokens != nil {
		for _, cached := range []*int64{usage.CacheReadTokens, usage.CacheWriteTokens} {
			if cached != nil && *cached > *usage.InputTokens {
				return Event{}, errors.New("cache token counts must not exceed inclusive input_tokens")
			}
		}
	}
	encoded, err := json.Marshal(usage)
	if err != nil {
		return Event{}, err
	}
	var data map[string]any
	if err := json.Unmarshal(encoded, &data); err != nil {
		return Event{}, err
	}
	return s.recordEvent(ctx, RecordEventInput{TaskID: input.TaskID, Actor: input.Actor, Kind: "usage", Stage: input.Stage, Attempt: input.Attempt, Message: "Model usage recorded", Data: data, RequestKey: input.RequestKey})
}

func (s *Store) recordEvent(ctx context.Context, input RecordEventInput) (Event, error) {
	if err := required("task_id", input.TaskID, "actor", input.Actor, "message", input.Message); err != nil {
		return Event{}, err
	}
	if input.Attempt < 0 {
		return Event{}, errors.New("attempt must not be negative")
	}
	if input.Data == nil {
		input.Data = map[string]any{}
	}
	encoded, err := json.Marshal(input.Data)
	if err != nil {
		return Event{}, err
	}
	if len(encoded) > 1<<20 {
		return Event{}, errors.New("event data exceeds 1 MiB")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, err
	}
	defer tx.Rollback()
	if input.RequestKey != "" {
		old, err := scanEvent(tx.QueryRowContext(ctx, "SELECT "+eventColumns+" FROM events WHERE task_id=? AND actor=? AND request_key=?", input.TaskID, input.Actor, input.RequestKey))
		if err == nil {
			oldData, err := json.Marshal(old.Data)
			if err != nil {
				return Event{}, err
			}
			if old.Kind != input.Kind || old.Stage != input.Stage || old.Attempt != input.Attempt || old.Message != input.Message || string(oldData) != string(encoded) {
				return Event{}, errors.New("request_key was already used for a different event")
			}
			return old, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Event{}, err
		}
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO events(task_id,actor,kind,stage,attempt,message,data,created_at,request_key) VALUES(?,?,?,?,?,?,?,?,NULLIF(?,''))", input.TaskID, input.Actor, input.Kind, input.Stage, input.Attempt, input.Message, string(encoded), now(), input.RequestKey)
	if err != nil {
		return Event{}, fmt.Errorf("save event (task must exist): %w", err)
	}
	sequence, err := result.LastInsertId()
	if err != nil {
		return Event{}, err
	}
	event, err := scanEvent(tx.QueryRowContext(ctx, "SELECT "+eventColumns+" FROM events WHERE sequence=?", sequence))
	if err != nil {
		return Event{}, err
	}
	return event, tx.Commit()
}

func (s *Store) Events(ctx context.Context, input ListEventsInput) ([]Event, error) {
	if _, err := s.Task(ctx, input.TaskID); err != nil {
		return nil, err
	}
	limit, err := pageLimit(input.Limit, 100, 500)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+eventColumns+" FROM events WHERE task_id=? AND sequence>? ORDER BY sequence LIMIT ?", input.TaskID, input.After, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
