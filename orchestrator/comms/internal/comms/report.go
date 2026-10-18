package comms

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"sort"
	"time"
)

type UsageTotals struct {
	Records       int      `json:"records"`
	InputRecords  int      `json:"input_records"`
	OutputRecords int      `json:"output_records"`
	CostRecords   int      `json:"cost_records"`
	InputTokens   *int64   `json:"input_tokens"`
	OutputTokens  *int64   `json:"output_tokens"`
	TotalTokens   *int64   `json:"total_tokens"`
	CostUSD       *float64 `json:"cost_usd"`
	Models        []string `json:"models"`
	Sources       []string `json:"sources"`
}

func (t *UsageTotals) add(usage Usage) {
	t.Records++
	if usage.InputTokens != nil {
		if t.InputTokens == nil {
			t.InputTokens = new(int64)
		}
		*t.InputTokens += *usage.InputTokens
		t.InputRecords++
	}
	if usage.OutputTokens != nil {
		if t.OutputTokens == nil {
			t.OutputTokens = new(int64)
		}
		*t.OutputTokens += *usage.OutputTokens
		t.OutputRecords++
	}
	if usage.CostUSD != nil {
		if t.CostUSD == nil {
			t.CostUSD = new(float64)
		}
		*t.CostUSD += *usage.CostUSD
		t.CostRecords++
	}
	if t.InputRecords == t.Records && t.OutputRecords == t.Records {
		total := *t.InputTokens + *t.OutputTokens
		t.TotalTokens = &total
	} else {
		t.TotalTokens = nil
	}
	t.Models = unique(t.Models, usage.Provider+"/"+usage.Model)
	t.Sources = unique(t.Sources, usage.Source)
}

func unique(values []string, value string) []string {
	for _, old := range values {
		if old == value {
			return values
		}
	}
	return append(values, value)
}

type StageSummary struct {
	Name            string      `json:"name"`
	Attempt         int         `json:"attempt"`
	Status          string      `json:"status"`
	StartedAt       string      `json:"started_at,omitempty"`
	FinishedAt      string      `json:"finished_at,omitempty"`
	DurationSeconds *float64    `json:"duration_seconds"`
	Failures        int         `json:"failures"`
	Usage           UsageTotals `json:"usage"`
}

type TaskReport struct {
	Deliveries             []Delivery      `json:"deliveries"`
	DeliveryCounts         map[string]int  `json:"delivery_counts"`
	FailedResults          int             `json:"failed_results"`
	Participants           []Participant   `json:"participants"`
	ParticipantCount       int             `json:"participant_count"`
	Assignments            []Assignment    `json:"assignments"`
	AssignmentCount        int             `json:"assignment_count"`
	AssignmentCounts       map[string]int  `json:"assignment_counts"`
	Task                   Task            `json:"task"`
	GeneratedAt            string          `json:"generated_at"`
	EventCounts            map[string]int  `json:"event_counts"`
	EventCount             int             `json:"event_count"`
	MessageCount           int             `json:"message_count"`
	UnacknowledgedMessages int             `json:"unacknowledged_messages"`
	Usage                  UsageTotals     `json:"usage"`
	Stages                 []*StageSummary `json:"stages"`
	ReportPath             string          `json:"report_path"`
	// The browser timeline is bounded; totals and usage cover the entire task.
	Events   []Event   `json:"-"`
	Messages []Message `json:"-"`
}

func (s *Store) Report(ctx context.Context, id string) (TaskReport, error) {
	report := TaskReport{Participants: []Participant{}, Assignments: []Assignment{}, AssignmentCounts: map[string]int{}, GeneratedAt: now(), EventCounts: map[string]int{}, Stages: []*StageSummary{}, Events: []Event{}, Messages: []Message{}, ReportPath: "/tasks/" + url.PathEscape(id) + "/report.html"}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	report.Task, err = scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", id))
	if err != nil {
		return report, err
	}
	counts, err := tx.QueryContext(ctx, "SELECT kind,count(*) FROM events WHERE task_id=? GROUP BY kind", id)
	if err != nil {
		return report, err
	}
	for counts.Next() {
		var kind string
		var count int
		if err := counts.Scan(&kind, &count); err != nil {
			counts.Close()
			return report, err
		}
		report.EventCounts[kind] = count
		report.EventCount += count
	}
	err = counts.Err()
	counts.Close()
	if err != nil {
		return report, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT count(*),coalesce(sum(acknowledged_at IS NULL),0) FROM messages WHERE task_id=?", id).Scan(&report.MessageCount, &report.UnacknowledgedMessages); err != nil {
		return report, err
	}
	report.Deliveries, err = deliveries(ctx, tx, id)
	if err != nil {
		return report, err
	}
	report.DeliveryCounts = map[string]int{}
	deliveryCounts, err := tx.QueryContext(ctx, "SELECT state,count(*) FROM deliveries WHERE task_id=? GROUP BY state", id)
	if err != nil {
		return report, err
	}
	for deliveryCounts.Next() {
		var state string
		var count int
		if err := deliveryCounts.Scan(&state, &count); err != nil {
			deliveryCounts.Close()
			return report, err
		}
		report.DeliveryCounts[state] = count
	}
	err = deliveryCounts.Err()
	deliveryCounts.Close()
	if err != nil {
		return report, err
	}
	report.Participants, err = participants(ctx, tx, ListParticipantsInput{TaskID: id, Limit: 200})
	if err != nil {
		return report, err
	}
	report.Assignments, err = assignments(ctx, tx, ListAssignmentsInput{TaskID: id, Limit: 200})
	if err != nil {
		return report, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM participants WHERE task_id=?", id).Scan(&report.ParticipantCount); err != nil {
		return report, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM assignments WHERE task_id=? AND json_extract(result,'$.outcome')='failed'", id).Scan(&report.FailedResults); err != nil {
		return report, err
	}

	states, err := tx.QueryContext(ctx, "SELECT state,count(*) FROM assignments WHERE task_id=? GROUP BY state", id)
	if err != nil {
		return report, err
	}
	for states.Next() {
		var state string
		var count int
		if err := states.Scan(&state, &count); err != nil {
			states.Close()
			return report, err
		}
		report.AssignmentCounts[state] = count
		report.AssignmentCount += count
	}
	err = states.Err()
	states.Close()
	if err != nil {
		return report, err
	}

	stages := map[string]*StageSummary{}
	rows, err := tx.QueryContext(ctx, "SELECT "+eventColumns+" FROM events WHERE task_id=? AND (kind='usage' OR (stage<>'' AND kind IN ('stage_started','stage_completed','failure','assignment_created','assignment_updated','assignment_submitted'))) ORDER BY sequence", id)
	if err != nil {
		return report, err
	}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			rows.Close()
			return report, err
		}
		key := fmt.Sprintf("%s\x00%d", event.Stage, event.Attempt)
		stage := stages[key]
		if stage == nil {
			stage = &StageSummary{Name: event.Stage, Attempt: event.Attempt, Status: "recorded"}
			stages[key] = stage
			report.Stages = append(report.Stages, stage)
		}
		switch event.Kind {
		case "usage":
			var usage Usage
			encoded, err := json.Marshal(event.Data)
			if err != nil {
				rows.Close()
				return report, err
			}
			if err := json.Unmarshal(encoded, &usage); err != nil {
				rows.Close()
				return report, fmt.Errorf("read usage event %d: %w", event.Sequence, err)
			}
			stage.Usage.add(usage)
			report.Usage.add(usage)
		case "assignment_created", "assignment_updated", "assignment_submitted":
			if state, ok := event.Data["state"].(string); ok {
				stage.Status = state
				if state == "working" && stage.StartedAt == "" {
					stage.StartedAt = event.CreatedAt
				}
				if oneOf(state, "submitted", "declined", "cancelled") {
					stage.FinishedAt = event.CreatedAt
				}
			}
		case "stage_started":
			if stage.StartedAt == "" {
				stage.StartedAt = event.CreatedAt
			}
			stage.Status = "working"
		case "stage_completed":
			stage.FinishedAt = event.CreatedAt
			stage.Status = "completed"
		case "failure":
			stage.Failures++
			stage.Status = "failed"
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	for _, stage := range report.Stages {
		start, startErr := time.Parse(time.RFC3339Nano, stage.StartedAt)
		end, endErr := time.Parse(time.RFC3339Nano, stage.FinishedAt)
		if startErr == nil && endErr == nil && !end.Before(start) {
			duration := end.Sub(start).Seconds()
			stage.DurationSeconds = &duration
		}
	}
	rows, err = tx.QueryContext(ctx, "SELECT "+eventColumns+" FROM (SELECT "+eventColumns+" FROM events WHERE task_id=? ORDER BY sequence DESC LIMIT 500) ORDER BY sequence", id)
	if err != nil {
		return report, err
	}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			rows.Close()
			return report, err
		}
		report.Events = append(report.Events, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT "+messageColumns+" FROM (SELECT "+messageColumns+" FROM messages WHERE task_id=? ORDER BY id DESC LIMIT 200) ORDER BY id", id)
	if err != nil {
		return report, err
	}
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			rows.Close()
			return report, err
		}
		report.Messages = append(report.Messages, message)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	return report, tx.Commit()
}

//go:embed report.html
var reportHTML string

type stageChart struct {
	*StageSummary
	InputWidth  int
	OutputWidth int
}

var reportTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"tokens": func(value *int64) string {
		if value == nil {
			return "Unknown"
		}
		return fmt.Sprint(*value)
	},
	"cost": func(value *float64) string {
		if value == nil {
			return "Unknown"
		}
		return fmt.Sprintf("$%.6f", *value)
	},
	"duration": func(value *float64) string {
		if value == nil {
			return "Unknown"
		}
		return fmt.Sprintf("%.2fs", *value)
	},
	"json": func(value any) string {
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "Invalid evidence data"
		}
		return string(encoded)
	},
}).Parse(reportHTML))

func RenderReport(writer io.Writer, report TaskReport) error {
	charts := []stageChart{}
	var maximum int64 = 1
	for _, stage := range report.Stages {
		if stage.Usage.InputTokens != nil && *stage.Usage.InputTokens > maximum {
			maximum = *stage.Usage.InputTokens
		}
		if stage.Usage.OutputTokens != nil && *stage.Usage.OutputTokens > maximum {
			maximum = *stage.Usage.OutputTokens
		}
	}
	for _, stage := range report.Stages {
		chart := stageChart{StageSummary: stage}
		if stage.Usage.InputTokens != nil {
			chart.InputWidth = int(float64(*stage.Usage.InputTokens) / float64(maximum) * 100)
		}
		if stage.Usage.OutputTokens != nil {
			chart.OutputWidth = int(float64(*stage.Usage.OutputTokens) / float64(maximum) * 100)
		}
		charts = append(charts, chart)
	}
	sort.Strings(report.Usage.Models)
	return reportTemplate.Execute(writer, struct {
		TaskReport
		Charts []stageChart
	}{report, charts})
}
