package comms

// Participant identifies one declared agent instance within a task. Registration
// is not authentication or proof that its process is still reachable.
type Participant struct {
	ID              string  `json:"id"`
	TaskID          string  `json:"task_id"`
	InstanceID      string  `json:"instance_id"`
	Role            string  `json:"role"`
	RoutingIdentity string  `json:"routing_identity"`
	SessionRef      string  `json:"session_ref,omitempty"`
	RegisteredAt    string  `json:"registered_at"`
	RetiredAt       *string `json:"retired_at,omitempty"`
}

type RegisterParticipantInput struct {
	ReplacesParticipantID string `json:"replaces_participant_id,omitempty" jsonschema:"Explicitly retire the prior instance with the same role and routing identity; assignments are not transferred"`
	TaskID                string `json:"task_id"`
	InstanceID            string `json:"instance_id" jsonschema:"Stable identity for this process/session, retained across context resets"`
	Role                  string `json:"role"`
	RoutingIdentity       string `json:"routing_identity" jsonschema:"Unique inbox recipient within this task; use the task owner identity for its orchestrator"`
	SessionRef            string `json:"session_ref,omitempty" jsonschema:"Actual optional host session reference; not a guessed pane ID"`
	RequestKey            string `json:"request_key"`
}

type ListParticipantsInput struct {
	TaskID string `json:"task_id"`
	After  string `json:"after,omitempty"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Default 50, maximum 200"`
}

// Binding changes describe the same process. A new process still requires an
// explicit replacement and must not inherit this participant's assignments.
type BindParticipantInput struct {
	TaskID             string `json:"task_id"`
	ParticipantID      string `json:"participant_id"`
	Actor              string `json:"actor_participant_id"`
	ExpectedSessionRef string `json:"expected_session_ref"`
	SessionRef         string `json:"session_ref" jsonschema:"Captured host reference for this same instance; empty disables background delivery"`
	RequestKey         string `json:"request_key"`
}

type Assignment struct {
	ID                   string            `json:"id"`
	TaskID               string            `json:"task_id"`
	AssigningParticipant string            `json:"assigning_participant_id"`
	Assignee             string            `json:"assignee_id"`
	Title                string            `json:"title"`
	Instructions         string            `json:"instructions"`
	Inputs               []string          `json:"inputs"`
	ExpectedOutput       []string          `json:"expected_output"`
	Stage                string            `json:"stage"`
	Attempt              int               `json:"attempt"`
	State                string            `json:"state"`
	Summary              string            `json:"summary"`
	Version              int64             `json:"version"`
	CreatedAt            string            `json:"created_at"`
	UpdatedAt            string            `json:"updated_at"`
	Result               *AssignmentResult `json:"result,omitempty"`
}

type CheckEvidence struct {
	Command    string `json:"command"`
	ExitStatus int    `json:"exit_status"`
	OutputRef  string `json:"output_ref,omitempty"`
}

// A result contains submitted claims, not independently verified work.
type AssignmentResult struct {
	Outcome     string          `json:"outcome" jsonschema:"ready_for_review, completed, or failed; does not change overall task status"`
	Summary     string          `json:"summary"`
	Artifacts   []string        `json:"artifacts,omitempty"`
	Checks      []CheckEvidence `json:"checks,omitempty"`
	Limitations []string        `json:"limitations,omitempty"`
	NextAction  string          `json:"next_action"`
}

type CreateAssignmentInput struct {
	TaskID               string   `json:"task_id"`
	AssigningParticipant string   `json:"assigning_participant_id"`
	Assignee             string   `json:"assignee_id"`
	Title                string   `json:"title"`
	Instructions         string   `json:"instructions"`
	Inputs               []string `json:"inputs,omitempty"`
	ExpectedOutput       []string `json:"expected_output"`
	Stage                string   `json:"stage"`
	Attempt              int      `json:"attempt"`
	RequestKey           string   `json:"request_key"`
}

type AssignmentInput struct {
	TaskID       string `json:"task_id"`
	AssignmentID string `json:"assignment_id"`
}

type ListAssignmentsInput struct {
	TaskID   string `json:"task_id"`
	Assignee string `json:"assignee_id,omitempty"`
	State    string `json:"state,omitempty"`
	After    string `json:"after,omitempty"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Default 50, maximum 200"`
}

type UpdateAssignmentInput struct {
	TaskID          string `json:"task_id"`
	AssignmentID    string `json:"assignment_id"`
	Actor           string `json:"actor_participant_id"`
	ExpectedVersion int64  `json:"expected_version"`
	State           string `json:"state" jsonschema:"accepted, declined, working, blocked, needs_input, or cancelled; results use submit_result"`
	Summary         string `json:"summary"`
	RequestKey      string `json:"request_key"`
}

type SubmitResultInput struct {
	TaskID          string           `json:"task_id"`
	AssignmentID    string           `json:"assignment_id"`
	Actor           string           `json:"actor_participant_id"`
	ExpectedVersion int64            `json:"expected_version"`
	Result          AssignmentResult `json:"result"`
	RequestKey      string           `json:"request_key"`
}
