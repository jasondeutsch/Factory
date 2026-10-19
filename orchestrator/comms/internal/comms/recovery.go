package comms

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type RecoveryPlan struct {
	Assignment  Assignment       `json:"assignment"`
	Participant Participant      `json:"participant"`
	Session     *SessionSnapshot `json:"session,omitempty"`
	Actions     []string         `json:"actions"`
	Warning     string           `json:"warning"`
}
type RecoverAssignmentInput struct {
	TaskID                 string `json:"task_id"`
	AssignmentID           string `json:"assignment_id"`
	Actor                  string `json:"actor_participant_id"`
	ExpectedVersion        int64  `json:"expected_version"`
	Action                 string `json:"action" jsonschema:"cancel or reassign"`
	ReplacementID          string `json:"replacement_participant_id,omitempty"`
	PreviousSessionStopped bool   `json:"previous_session_stopped" jsonschema:"Explicit operator confirmation, not inferred from expired contact; required for reassignment"`
	Reason                 string `json:"reason"`
	RequestKey             string `json:"request_key"`
}
type RecoveryResult struct {
	Previous    Assignment  `json:"previous"`
	Replacement *Assignment `json:"replacement,omitempty"`
}

func (s *Store) RecoveryPlan(ctx context.Context, in AssignmentInput) (RecoveryPlan, error) {
	a, err := s.Assignment(ctx, in)
	if err != nil {
		return RecoveryPlan{}, err
	}
	p, err := participantRecord(ctx, s.db, in.TaskID, a.Assignee)
	if err != nil {
		return RecoveryPlan{}, err
	}
	out := RecoveryPlan{Assignment: a, Participant: p, Actions: []string{}, Warning: "Contact expiry does not prove the process stopped. Stop it through native controls before reassigning. Saved inputs, summary, messages and checks remain evidence, not verified completion."}
	if validAssignmentTransition(a.State, "cancelled") {
		out.Actions = []string{"cancel", "reassign"}
	}
	snapshot, err := s.Session(ctx, SessionInput{TaskID: in.TaskID, ParticipantID: p.ID})
	if err == nil {
		out.Session = &snapshot
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	return out, nil
}
func (s *Store) RecoverAssignment(ctx context.Context, in RecoverAssignmentInput) (RecoveryResult, error) {
	if err := required("reason", in.Reason); err != nil {
		return RecoveryResult{}, err
	}
	if !oneOf(in.Action, "cancel", "reassign") {
		return RecoveryResult{}, errors.New("action must be cancel or reassign")
	}
	if in.Action == "reassign" && (!in.PreviousSessionStopped || in.ReplacementID == "") {
		return RecoveryResult{}, errors.New("reassignment requires an actual replacement participant and explicit confirmation that the previous session stopped")
	}
	if in.Action == "cancel" && in.ReplacementID != "" {
		return RecoveryResult{}, errors.New("cancel does not take a replacement participant")
	}
	return coordinationMutation(ctx, s, in.TaskID, "participant:"+in.Actor, in.RequestKey, "recover_assignment", in, func(tx *sql.Tx) (RecoveryResult, error) {
		var out RecoveryResult
		owner, err := activeParticipant(ctx, tx, in.TaskID, in.Actor)
		if err != nil {
			return out, err
		}
		task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", in.TaskID))
		if err != nil {
			return out, err
		}
		if owner.RoutingIdentity != task.Owner {
			return out, errors.New("only the task owner can recover an assignment")
		}
		a, err := assignmentRecord(ctx, tx, in.TaskID, in.AssignmentID)
		if err != nil {
			return out, err
		}
		if a.Version != in.ExpectedVersion || !validAssignmentTransition(a.State, "cancelled") {
			return out, errors.New("stale or terminal assignment; inspect before recovery")
		}
		old, err := participantRecord(ctx, tx, in.TaskID, a.Assignee)
		if err != nil {
			return out, err
		}
		if in.Action == "reassign" {
			if err := requireActiveTask(ctx, tx, in.TaskID, false); err != nil {
				return out, err
			}
			replacement, err := activeParticipant(ctx, tx, in.TaskID, in.ReplacementID)
			if err != nil {
				return out, err
			}
			if replacement.ID == old.ID {
				return out, errors.New("replacement must be a different actual instance")
			}
			inputs, _ := json.Marshal(append(append([]string{}, a.Inputs...), "assignment:"+a.ID))
			expected, _ := json.Marshal(a.ExpectedOutput)
			handoff, _ := json.Marshal(a.Handoff)
			id, at := newID(), now()
			_, err = tx.ExecContext(ctx, "INSERT INTO assignments(id,task_id,assigning_participant_id,assignee_id,title,instructions,inputs,expected_output,stage,attempt,state,summary,version,created_at,updated_at,handoff) VALUES(?,?,?,?,?,?,?,?,?,?,'offered',?,1,?,?,?)", id, in.TaskID, owner.ID, replacement.ID, a.Title, a.Instructions, string(inputs), string(expected), a.Stage, a.Attempt+1, "Recovery: "+in.Reason, at, at, string(handoff))
			if err != nil {
				return out, err
			}
			next, err := assignmentRecord(ctx, tx, in.TaskID, id)
			if err != nil {
				return out, err
			}
			if err := publishAssignment(ctx, tx, next, owner.RoutingIdentity, replacement.RoutingIdentity, "assignment", "assignment_created"); err != nil {
				return out, err
			}
			out.Replacement = &next
		}
		_, err = tx.ExecContext(ctx, "UPDATE assignments SET state='cancelled',version=version+1,updated_at=? WHERE id=?", now(), a.ID)
		if err != nil {
			return out, err
		}
		out.Previous, err = assignmentRecord(ctx, tx, in.TaskID, a.ID)
		if err != nil {
			return out, err
		}
		if err := publishAssignment(ctx, tx, out.Previous, owner.RoutingIdentity, old.RoutingIdentity, "progress", "assignment_updated"); err != nil {
			return out, err
		}
		data := map[string]any{"assignment_id": a.ID, "action": in.Action, "previous_session_stopped": in.PreviousSessionStopped}
		if out.Replacement != nil {
			data["replacement_assignment_id"] = out.Replacement.ID
			data["replacement_participant_id"] = in.ReplacementID
		}
		err = appendEvent(ctx, tx, in.TaskID, owner.RoutingIdentity, "assignment_recovered", in.Reason, data)
		return out, err
	})
}
