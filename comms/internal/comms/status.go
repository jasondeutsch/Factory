package comms

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"io"
)

type FactoryMember struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
type FactorySnapshot struct {
	Name   string          `json:"name"`
	Owner  string          `json:"owner"`
	Agents []FactoryMember `json:"agents"`
}
type SetTaskFactoryInput struct {
	TaskID  string          `json:"task_id"`
	Factory FactorySnapshot `json:"factory"`
}

func (s *Store) SetTaskFactory(ctx context.Context, in SetTaskFactoryInput) (FactorySnapshot, error) {
	if err := required("factory.name", in.Factory.Name, "factory.owner", in.Factory.Owner); err != nil {
		return FactorySnapshot{}, err
	}
	names := map[string]bool{}
	for _, m := range in.Factory.Agents {
		if err := required("member.name", m.Name, "member.path", m.Path); err != nil {
			return FactorySnapshot{}, err
		}
		if names[m.Name] {
			return FactorySnapshot{}, errors.New("duplicate factory member")
		}
		names[m.Name] = true
	}
	if !names[in.Factory.Owner] {
		return FactorySnapshot{}, errors.New("factory owner must be a member")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FactorySnapshot{}, err
	}
	defer tx.Rollback()
	task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", in.TaskID))
	if err != nil {
		return FactorySnapshot{}, err
	}
	if task.Owner != in.Factory.Owner {
		return FactorySnapshot{}, errors.New("factory owner differs from task owner")
	}
	encoded, _ := json.Marshal(in.Factory)
	var old string
	err = tx.QueryRowContext(ctx, "SELECT snapshot FROM task_factories WHERE task_id=?", in.TaskID).Scan(&old)
	if err == nil {
		if old != string(encoded) {
			return FactorySnapshot{}, errors.New("task factory snapshot is immutable")
		}
		return in.Factory, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return FactorySnapshot{}, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO task_factories(task_id,snapshot) VALUES(?,?)", in.TaskID, string(encoded))
	if err != nil {
		return FactorySnapshot{}, err
	}
	return in.Factory, tx.Commit()
}

type MemberStatus struct {
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	Instances []Participant `json:"instances"`
}
type TaskStatus struct {
	Inbox    CoordinatorInbox `json:"inbox"`
	Report   TaskReport       `json:"report"`
	Factory  *FactorySnapshot `json:"factory,omitempty"`
	Members  []MemberStatus   `json:"members"`
	Events   []Event          `json:"events"`
	Messages []Message        `json:"messages"`
	Partial  bool             `json:"partial"`
}

func (s *Store) Status(ctx context.Context, in TaskInput) (TaskStatus, error) {
	report, err := s.Report(ctx, in.TaskID)
	if err != nil {
		return TaskStatus{}, err
	}
	out := TaskStatus{Report: report, Members: []MemberStatus{}, Events: report.Events, Messages: report.Messages,
		Partial: report.ParticipantCount > len(report.Participants) || report.AssignmentCount > len(report.Assignments)}
	var encoded string
	err = s.db.QueryRowContext(ctx, "SELECT snapshot FROM task_factories WHERE task_id=?", in.TaskID).Scan(&encoded)
	if err == nil {
		out.Factory = new(FactorySnapshot)
		if err = json.Unmarshal([]byte(encoded), out.Factory); err != nil {
			return out, err
		}
		for _, m := range out.Factory.Agents {
			member := MemberStatus{Name: m.Name, Path: m.Path, Instances: []Participant{}}
			for _, p := range report.Participants {
				if p.Role == m.Name {
					member.Instances = append(member.Instances, p)
				}
			}
			out.Members = append(out.Members, member)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	out.Inbox, err = s.InboxSummary(ctx, in)
	return out, err
}

//go:embed status.html
var statusHTML string
var statusTemplate = template.Must(template.New("status").Funcs(template.FuncMap{"participant": func(items []Participant, id string) string {
	for _, p := range items {
		if p.ID == id {
			return p.RoutingIdentity
		}
	}
	return id
}, "json": func(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }}).Parse(statusHTML))

func RenderStatus(w io.Writer, status TaskStatus) error { return statusTemplate.Execute(w, status) }
