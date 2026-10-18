package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"factory/comms/internal/comms"
)

type Caller interface {
	Call(context.Context, string, any, any) error
}
type Host interface {
	Ready(context.Context, Binding) (bool, error)
	Prompt(context.Context, Binding, string) error
}
type Bridge struct {
	Client  Caller
	Host    Host
	Project string
	TaskID  string
	Logger  *slog.Logger
}

// Cycle pages all watched tasks/participants/inboxes, sending at most one hint
// per participant. It does not read message bodies into a model or mark them read.
func (b Bridge) Cycle(ctx context.Context) error {
	if b.Project == "" {
		return errors.New("project is required")
	}
	if b.TaskID != "" {
		var task comms.Task
		if err := b.Client.Call(ctx, "get_task", comms.TaskInput{TaskID: b.TaskID}, &task); err != nil {
			return err
		}
		if task.Project != b.Project {
			return errors.New("task belongs to another project")
		}
		return b.task(ctx, task)
	}
	var before int64
	for {
		var page struct {
			Tasks []comms.Task `json:"tasks"`
		}
		if err := b.Client.Call(ctx, "list_tasks", comms.ListTasksInput{Project: b.Project, Limit: 100, Before: before}, &page); err != nil {
			return err
		}
		for _, task := range page.Tasks {
			if err := b.task(ctx, task); err != nil {
				return err
			}
		}
		if len(page.Tasks) < 100 {
			return nil
		}
		before = page.Tasks[len(page.Tasks)-1].Sequence
	}
}
func (b Bridge) task(ctx context.Context, task comms.Task) error {
	var after string
	for {
		var page struct {
			Participants []comms.Participant `json:"participants"`
		}
		if err := b.Client.Call(ctx, "list_participants", comms.ListParticipantsInput{TaskID: task.ID, After: after, Limit: 200}, &page); err != nil {
			return err
		}
		for _, participant := range page.Participants {
			if participant.RetiredAt != nil {
				continue
			}
			if err := b.inbox(ctx, participant); err != nil {
				return err
			}
		}
		if len(page.Participants) < 200 {
			return nil
		}
		after = page.Participants[len(page.Participants)-1].ID
	}
}
func (b Bridge) inbox(ctx context.Context, p comms.Participant) error {
	if p.SessionRef == "" {
		return nil
	} // Explicit manual mode.
	var after string
	for {
		var page struct {
			Messages []comms.Message `json:"messages"`
		}
		if err := b.Client.Call(ctx, "read_inbox", comms.InboxInput{TaskID: p.TaskID, Recipient: p.RoutingIdentity, After: after, Limit: 200}, &page); err != nil {
			return err
		}
		if len(page.Messages) == 0 {
			return nil
		}
		var binding Binding
		bindingErr := json.Unmarshal([]byte(p.SessionRef), &binding)
		if bindingErr != nil {
			bindingErr = errors.New("session_ref is not a captured Herdr binding")
		} else {
			bindingErr = binding.Validate()
		}
		ready := false
		if bindingErr == nil {
			ready, bindingErr = b.Host.Ready(ctx, binding)
		}
		// Busy/blocked/unknown is not a failure and must never trigger input.
		if bindingErr == nil && !ready {
			return nil
		}
		for _, message := range page.Messages {
			claimInput := comms.ClaimDeliveryInput{TaskID: p.TaskID, ParticipantID: p.ID, MessageID: message.ID}
			var claim comms.DeliveryClaim
			if err := b.Client.Call(ctx, "claim_delivery", claimInput, &claim); err != nil {
				return err
			}
			if claim.Delivery == nil {
				continue
			}
			finish := comms.FinishDeliveryInput{ClaimDeliveryInput: claimInput, LeaseToken: claim.Delivery.LeaseToken, State: "submitted", Detail: "Herdr accepted inbox notification input; reading and assignment acceptance are still pending"}
			if bindingErr != nil {
				finish.State = "retry"
				finish.Detail = bindingErr.Error()
			} else {
				// Never paste stored message text: it may contain commands or untrusted
				// instructions. The recipient fetches canonical records through its MCP.
				hint := fmt.Sprintf("Factory inbox notification. Read comms read_inbox for task_id=%q, recipient=%q, message_id=%q. Inspect canonical assignment state before acting; acknowledge reading separately. Follow your task-comms skill.", p.TaskID, p.RoutingIdentity, message.ID)
				// Recheck immediately before input. Herdr's get/prompt are not atomic; see
				// documented host limitations. No interruption or raw-key fallback exists.
				current, err := b.Host.Ready(ctx, binding)
				if err != nil {
					finish.State = "retry"
					finish.Detail = err.Error()
				} else if !current {
					finish.State = "retry"
					finish.Detail = "Session became busy before notification; no input submitted"
				} else if err := b.Host.Prompt(ctx, binding, hint); err != nil {
					finish.State = "uncertain"
					finish.Detail = "Herdr input outcome uncertain: " + err.Error() + ". Inspect the pane before retrying."
				}
			}
			var result comms.Delivery
			if err := b.Client.Call(ctx, "finish_delivery", finish, &result); err != nil {
				return err
			}
			if b.Logger != nil {
				b.Logger.Info("inbox notification", "task", p.TaskID, "participant", p.ID, "message", message.ID, "state", result.State, "detail", result.Detail)
			}
			return nil
		}
		if len(page.Messages) < 200 {
			return nil
		}
		after = page.Messages[len(page.Messages)-1].ID
	}
}
