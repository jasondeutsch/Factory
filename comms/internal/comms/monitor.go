package comms

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
)

//go:embed monitor.html
var monitorHTML string

//go:embed monitor.js
var monitorJavaScript string

// MonitorTask is the compact, project-wide view used by the browser monitor.
// Stage comes from an active assignment; no stage is inferred from task text
// or participant contact.
type MonitorTask struct {
	Task             Task   `json:"task"`
	Factory          string `json:"factory,omitempty"`
	ActiveStage      string `json:"active_stage,omitempty"`
	ActiveAssignment string `json:"active_assignment_title,omitempty"`
	AssignmentState  string `json:"assignment_state,omitempty"`
	Assignee         string `json:"assignee,omitempty"`
	OpenAttention    int    `json:"open_attention"`
	UnreadMessages   int    `json:"unread_messages"`
	LatestEvent      string `json:"latest_event,omitempty"`
	LatestEventAt    string `json:"latest_event_at,omitempty"`
}

type MonitorTasksInput struct {
	Project string `json:"project"`
	Limit   int    `json:"limit,omitempty"`
	Before  int64  `json:"before,omitempty"`
}

func (s *Store) Projects(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT project FROM tasks ORDER BY project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []string{}
	for rows.Next() {
		var project string
		if err := rows.Scan(&project); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Store) MonitorTasks(ctx context.Context, input MonitorTasksInput) ([]MonitorTask, error) {
	if err := required("project", input.Project); err != nil {
		return nil, err
	}
	limit, err := pageLimit(input.Limit, 50, 100)
	if err != nil {
		return nil, err
	}
	query := `SELECT
		tasks.sequence,tasks.id,tasks.project,tasks.name,tasks.brief,tasks.owner,tasks.status,tasks.summary,tasks.version,tasks.created_at,tasks.updated_at,
		task_factories.snapshot,
		active.stage,active.title,active.state,assignee.role,
		(SELECT count(*) FROM attention WHERE attention.task_id=tasks.id AND attention.state='open'),
		(SELECT count(*) FROM messages WHERE messages.task_id=tasks.id AND messages.acknowledged_at IS NULL),
		latest.kind || ': ' || latest.message,latest.created_at
		FROM tasks
		LEFT JOIN task_factories ON task_factories.task_id=tasks.id
		LEFT JOIN assignments AS active ON active.id=(
			SELECT id FROM assignments
			WHERE task_id=tasks.id AND state IN ('offered','accepted','working','blocked','needs_input')
			ORDER BY updated_at DESC,id DESC LIMIT 1
		)
		LEFT JOIN participants AS assignee ON assignee.id=active.assignee_id
		LEFT JOIN events AS latest ON latest.sequence=(SELECT MAX(sequence) FROM events WHERE task_id=tasks.id)
		WHERE tasks.project=? AND (?=0 OR tasks.sequence<?)
		ORDER BY tasks.sequence DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, input.Project, input.Before, input.Before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []MonitorTask{}
	for rows.Next() {
		var item MonitorTask
		var snapshot sql.NullString
		var stage, title, state, assignee, latestEvent, latestAt sql.NullString
		if err := rows.Scan(&item.Task.Sequence, &item.Task.ID, &item.Task.Project, &item.Task.Name, &item.Task.Brief, &item.Task.Owner, &item.Task.Status, &item.Task.Summary, &item.Task.Version, &item.Task.CreatedAt, &item.Task.UpdatedAt,
			&snapshot, &stage, &title, &state, &assignee, &item.OpenAttention, &item.UnreadMessages, &latestEvent, &latestAt); err != nil {
			return nil, err
		}
		if snapshot.Valid {
			var factory FactorySnapshot
			if err := json.Unmarshal([]byte(snapshot.String), &factory); err != nil {
				return nil, err
			}
			item.Factory = factory.Name
		}
		item.ActiveStage = stage.String
		item.ActiveAssignment = title.String
		item.AssignmentState = state.String
		item.Assignee = assignee.String
		item.LatestEvent = latestEvent.String
		item.LatestEventAt = latestAt.String
		tasks = append(tasks, item)
	}
	return tasks, rows.Err()
}
