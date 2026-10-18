package comms

// Project is an explicit identifier supplied by the client, not a container path.
type Task struct {
	Sequence  int64  `json:"sequence"`
	ID        string `json:"id"`
	Project   string `json:"project"`
	Name      string `json:"name"`
	Brief     string `json:"brief"`
	Owner     string `json:"owner"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreateTaskInput struct {
	Project    string `json:"project" jsonschema:"Explicit project identifier shared by participating agents"`
	Name       string `json:"name"`
	Brief      string `json:"brief"`
	Owner      string `json:"owner" jsonschema:"Declared role responsible for task status"`
	RequestKey string `json:"request_key,omitempty" jsonschema:"Stable retry key unique within the project"`
}

type TaskInput struct {
	TaskID string `json:"task_id"`
}

type ListTasksInput struct {
	Project string `json:"project"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Default 20, maximum 100"`
	Before  int64  `json:"before,omitempty" jsonschema:"Return tasks with sequence lower than this cursor"`
}

type UpdateTaskInput struct {
	TaskID          string `json:"task_id"`
	Actor           string `json:"actor"`
	ExpectedVersion int64  `json:"expected_version" jsonschema:"Version returned by get_task; stale writes are rejected"`
	Status          string `json:"status" jsonschema:"pending, working, needs_input, blocked, complete, or cancelled"`
	Summary         string `json:"summary"`
}

type Message struct {
	AssignmentID   string  `json:"assignment_id,omitempty"`
	ID             string  `json:"id"`
	TaskID         string  `json:"task_id"`
	Sender         string  `json:"sender"`
	Recipient      string  `json:"recipient"`
	Kind           string  `json:"kind"`
	Text           string  `json:"text"`
	CreatedAt      string  `json:"created_at"`
	AcknowledgedAt *string `json:"acknowledged_at"`
}

type SendMessageInput struct {
	AssignmentID string `json:"assignment_id,omitempty" jsonschema:"Optional existing assignment in this task; does not change its state"`
	TaskID       string `json:"task_id"`
	Sender       string `json:"sender"`
	Recipient    string `json:"recipient"`
	Kind         string `json:"kind" jsonschema:"assignment, question, answer, progress, finding, result, or decision"`
	Text         string `json:"text"`
	RequestKey   string `json:"request_key,omitempty" jsonschema:"Stable retry key unique for this sender and task"`
}

type InboxInput struct {
	TaskID              string `json:"task_id"`
	Recipient           string `json:"recipient"`
	IncludeAcknowledged bool   `json:"include_acknowledged,omitempty"`
	After               string `json:"after,omitempty" jsonschema:"Message ID cursor from the previous page"`
	Limit               int    `json:"limit,omitempty" jsonschema:"Default 50, maximum 200"`
}

type AcknowledgeInput struct {
	TaskID    string `json:"task_id"`
	MessageID string `json:"message_id"`
	Actor     string `json:"actor"`
}

type Event struct {
	Sequence  int64          `json:"sequence"`
	TaskID    string         `json:"task_id"`
	Actor     string         `json:"actor"`
	Kind      string         `json:"kind"`
	Stage     string         `json:"stage,omitempty"`
	Attempt   int            `json:"attempt,omitempty"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data"`
	CreatedAt string         `json:"created_at"`
}

type RecordEventInput struct {
	TaskID     string         `json:"task_id"`
	Actor      string         `json:"actor"`
	Kind       string         `json:"kind" jsonschema:"progress, check, finding, failure, decision, note, stage_started, or stage_completed"`
	Stage      string         `json:"stage,omitempty"`
	Attempt    int            `json:"attempt,omitempty"`
	Message    string         `json:"message"`
	Data       map[string]any `json:"data,omitempty" jsonschema:"Structured evidence; decisions may include artifact paths and hashes; checks may include exit status"`
	RequestKey string         `json:"request_key,omitempty"`
}

type ListEventsInput struct {
	TaskID string `json:"task_id"`
	After  int64  `json:"after,omitempty"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Default 100, maximum 500"`
}

// Nil measurements mean unknown. Input includes cached input; output includes
// reasoning when the provider includes it. Cache counts are subsets, not extras.
type Usage struct {
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	Source           string   `json:"source" jsonschema:"runtime, provider, or agent_report; provenance is declared, not authenticated"`
	InputTokens      *int64   `json:"input_tokens,omitempty"`
	OutputTokens     *int64   `json:"output_tokens,omitempty"`
	CacheReadTokens  *int64   `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens *int64   `json:"cache_write_tokens,omitempty"`
	CostUSD          *float64 `json:"cost_usd,omitempty"`
}

type RecordUsageInput struct {
	TaskID     string `json:"task_id"`
	Actor      string `json:"actor"`
	Stage      string `json:"stage"`
	Attempt    int    `json:"attempt"`
	RequestKey string `json:"request_key" jsonschema:"Stable unique key for one measured model request; retries must reuse it"`
	Usage      Usage  `json:"usage"`
}
