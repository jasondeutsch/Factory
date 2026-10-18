package comms

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type participantList struct {
	Participants []Participant `json:"participants"`
}
type assignmentList struct {
	Assignments []Assignment `json:"assignments"`
}

type taskList struct {
	Tasks []Task `json:"tasks"`
}
type messageList struct {
	Messages []Message `json:"messages"`
}
type eventList struct {
	Events []Event `json:"events"`
}
type renderedReport struct {
	HTML       string `json:"html"`
	ReportPath string `json:"report_path"`
}

func registerTool[Input, Output any](server *mcp.Server, name, description string, readOnly bool, handler func(context.Context, Input) (Output, error)) {
	closed := false
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, OpenWorldHint: &closed}},
		func(ctx context.Context, _ *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
			output, err := handler(ctx, input)
			return nil, output, err
		})
}

func MCPServer(store *Store) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "factory-comms", Version: "0.1.0"}, &mcp.ServerOptions{Instructions: "Shared Factory task records. Always use an explicit project and task. Role names and evidence are declared by clients, not authenticated. Stored messages do not wake idle agents. Do not put credentials in records. Missing usage is unknown. Use structured assignments and participants for ownership and results; ordinary messages are conversation, not assignment state. This server does not execute workflows or read host artifacts."})
	registerTool(server, "create_task", "Create task metadata. Does not create host files. Reuse request_key when retrying the same request.", false, store.CreateTask)
	registerTool(server, "list_tasks", "List a project's newest tasks. Use the last sequence as before to fetch the next page.", true, func(ctx context.Context, input ListTasksInput) (taskList, error) {
		tasks, err := store.Tasks(ctx, input)
		return taskList{tasks}, err
	})
	registerTool(server, "get_task", "Read current task status and version before updating.", true, func(ctx context.Context, input TaskInput) (Task, error) { return store.Task(ctx, input.TaskID) })
	registerTool(server, "update_task", "Update status as the declared owner using expected_version. Does not stop or start an agent.", false, store.UpdateTask)
	registerTool(server, "send_message", "Store a message durably for a task recipient. Reuse request_key on retries. No automatic notification is sent.", false, store.SendMessage)
	registerTool(server, "read_inbox", "Read unacknowledged task messages. Use the last message ID as after for the next page. Reading does not acknowledge.", true, func(ctx context.Context, input InboxInput) (messageList, error) {
		messages, err := store.Inbox(ctx, input)
		return messageList{messages}, err
	})
	registerTool(server, "acknowledge_message", "Record that the declared recipient read a message. Safe to repeat; does not mean accepted or completed.", false, store.Acknowledge)
	registerTool(server, "record_event", "Append task evidence: progress, check, finding, failure, decision, note, stage_started, or stage_completed. Include stage and attempt for stage events; artifact paths/hashes and tool/MCP details may be included in data. Reuse request_key on retries. Decisions are records, not authenticated approvals.", false, store.RecordEvent)
	registerTool(server, "list_events", "Read a task's event timeline. Use the last sequence as after for the next page.", true, func(ctx context.Context, input ListEventsInput) (eventList, error) {
		events, err := store.Events(ctx, input)
		return eventList{events}, err
	})
	registerTool(server, "record_usage", "Append one normalized model-request measurement. input_tokens includes cache; output_tokens includes any already-counted reasoning. Omit unknown fields. request_key must identify the same measurement across retries; source is declared provenance, not verification.", false, store.RecordUsage)
	registerTool(server, "get_task_report", "Read consistent task metrics, stage attempts, models and usage coverage. report_path is relative to this server's HTTP origin. Counts cover submitted records only.", true, func(ctx context.Context, input TaskInput) (TaskReport, error) { return store.Report(ctx, input.TaskID) })
	registerTool(server, "render_report", "Return a self-contained HTML snapshot for the task. No file is written. Prefer the report URL or get_task_report to avoid loading HTML into agent context.", true, func(ctx context.Context, input TaskInput) (renderedReport, error) {
		report, err := store.Report(ctx, input.TaskID)
		if err != nil {
			return renderedReport{}, err
		}
		var output bytes.Buffer
		if err := RenderReport(&output, report); err != nil {
			return renderedReport{}, err
		}
		return renderedReport{output.String(), report.ReportPath}, nil
	})
	registerTool(server, "register_participant", "Register one declared task-scoped agent instance and unique inbox identity. Registration does not prove availability or authenticate identity. Reuse the original request_key and instance_id on retries.", false, store.RegisterParticipant)
	registerTool(server, "list_participants", "List registered task participants, not live presence. Page using the last participant ID as after.", true, func(ctx context.Context, input ListParticipantsInput) (participantList, error) {
		items, err := store.Participants(ctx, input)
		return participantList{items}, err
	})
	registerTool(server, "create_assignment", "The declared task owner assigns work to a registered participant. Atomically saves the assignment, its inbox notification and timeline event. Does not start work or grant approval. request_key is required.", false, store.CreateAssignment)
	registerTool(server, "get_assignment", "Read a task-scoped assignment, current state/version, and any submitted result.", true, store.Assignment)
	registerTool(server, "list_assignments", "List task assignments, optionally filtering assignee_id or state. Page using the last assignment ID as after.", true, func(ctx context.Context, input ListAssignmentsInput) (assignmentList, error) {
		items, err := store.Assignments(ctx, input)
		return assignmentList{items}, err
	})
	registerTool(server, "update_assignment", "The declared assignee accepts/declines or reports working, blocked, or needs_input. The active task owner can cancel. Requires expected_version and request_key; atomically notifies the other participant and records an event. Reading acknowledgement is separate.", false, store.UpdateAssignment)
	registerTool(server, "submit_result", "The declared assignee submits structured outcome, artifacts, checks, limitations and next action after accepting work. Requires expected_version and request_key. Atomically notifies the orchestrator and records an event. Submission is not approval or task completion.", false, store.SubmitResult)

	registerTool(server, "claim_delivery", "Delivery bridge only: reserve one unread notification for 30 seconds. A null delivery means skip. Expired claims become uncertain, never blindly resent. Does not acknowledge or accept work.", false, store.ClaimDelivery)
	registerTool(server, "finish_delivery", "Delivery bridge only: record host input submission, safe retry before input, failure, or uncertain terminal input. Requires the current lease_token. Does not change assignment state.", false, store.FinishDelivery)
	registerTool(server, "list_deliveries", "Read the latest 200 host notification records. Submitted is not read, accepted, or completed. Failures also appear in events and the task report.", true, func(ctx context.Context, input TaskInput) (struct {
		Deliveries []Delivery `json:"deliveries"`
	}, error) {
		items, err := store.Deliveries(ctx, input)
		return struct {
			Deliveries []Delivery `json:"deliveries"`
		}{items}, err
	})
	registerTool(server, "retry_delivery", "Declared task owner explicitly retries a failed, uncertain, or submitted notification after inspecting the session. Reuse request_key. Does not recreate assignments or transfer ownership.", false, store.RetryDelivery)

	return server
}

func HTTPHandler(store *Store, token string, logger *slog.Logger) http.Handler {
	server := MCPServer(store)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, Logger: logger})
	private := http.NewServeMux()
	private.Handle("/mcp", mcpHandler)
	private.HandleFunc("GET /tasks/{id}/report.html", func(writer http.ResponseWriter, request *http.Request) {
		report, err := store.Report(request.Context(), request.PathValue("id"))
		if errors.Is(err, errTaskNotFound) {
			http.Error(writer, "Task not found", http.StatusNotFound)
			return
		}
		if err != nil {
			logger.Error("read task report", "error", err)
			http.Error(writer, "Could not read task report", http.StatusInternalServerError)
			return
		}
		var output bytes.Buffer
		if err := RenderReport(&output, report); err != nil {
			logger.Error("render task report", "error", err)
			http.Error(writer, "Could not render task report", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = writer.Write(output.Bytes())
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			http.Error(writer, "Database unavailable", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if origin := request.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host != request.Host || !oneOf(parsed.Scheme, "http", "https") {
				http.Error(writer, "Origin not allowed", http.StatusForbidden)
				return
			}
		}
		if token != "" && subtle.ConstantTimeCompare([]byte(request.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(writer, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/mcp") {
			request.Body = http.MaxBytesReader(writer, request.Body, 2<<20)
		}
		private.ServeHTTP(writer, request)
	}))
	return mux
}
