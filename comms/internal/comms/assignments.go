package comms

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

func (s *Store) CreateAssignment(ctx context.Context, input CreateAssignmentInput) (Assignment, error) {
	if err := required("assigning_participant_id", input.AssigningParticipant, "assignee_id", input.Assignee, "title", input.Title, "instructions", input.Instructions, "stage", input.Stage); err != nil {
		return Assignment{}, err
	}
	if input.Attempt < 1 || len(input.ExpectedOutput) == 0 {
		return Assignment{}, errors.New("assignment requires a positive attempt and expected_output")
	}
	for _, value := range append(append([]string{}, input.Inputs...), input.ExpectedOutput...) {
		if err := required("input/output reference", value); err != nil {
			return Assignment{}, err
		}
	}
	if input.Inputs == nil {
		input.Inputs = []string{}
	}
	return coordinationMutation(ctx, s, input.TaskID, "participant:"+input.AssigningParticipant, input.RequestKey, "create_assignment", input, func(tx *sql.Tx) (Assignment, error) {
		assigner, err := activeParticipant(ctx, tx, input.TaskID, input.AssigningParticipant)
		if err != nil {
			return Assignment{}, err
		}
		task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", input.TaskID))
		if err != nil {
			return Assignment{}, err
		}
		if assigner.RoutingIdentity != task.Owner {
			return Assignment{}, errors.New("only the declared task owner can create assignments")
		}
		if task.Status == "cancelled" || task.Status == "complete" {
			return Assignment{}, errors.New("cannot assign work on a complete or cancelled task")
		}
		assignee, err := activeParticipant(ctx, tx, input.TaskID, input.Assignee)
		if err != nil {
			return Assignment{}, err
		}
		id, at := newID(), now()
		inputs, _ := json.Marshal(input.Inputs)
		expected, _ := json.Marshal(input.ExpectedOutput)
		_, err = tx.ExecContext(ctx, "INSERT INTO assignments(id,task_id,assigning_participant_id,assignee_id,title,instructions,inputs,expected_output,stage,attempt,state,summary,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,'offered',?,1,?,?)", id, input.TaskID, assigner.ID, assignee.ID, input.Title, input.Instructions, string(inputs), string(expected), input.Stage, input.Attempt, input.Title, at, at)
		if err != nil {
			return Assignment{}, err
		}
		assignment, err := assignmentRecord(ctx, tx, input.TaskID, id)
		if err != nil {
			return assignment, err
		}
		err = publishAssignment(ctx, tx, assignment, assigner.RoutingIdentity, assignee.RoutingIdentity, "assignment", "assignment_created")
		return assignment, err
	})
}

func (s *Store) Assignment(ctx context.Context, input AssignmentInput) (Assignment, error) {
	return assignmentRecord(ctx, s.db, input.TaskID, input.AssignmentID)
}

func assignmentState(state string) bool {
	return oneOf(state, "offered", "accepted", "working", "blocked", "needs_input", "submitted", "declined", "cancelled")
}

func assignments(ctx context.Context, reader databaseReader, input ListAssignmentsInput) ([]Assignment, error) {
	if _, err := scanTask(reader.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", input.TaskID)); err != nil {
		return nil, err
	}
	if input.State != "" && !assignmentState(input.State) {
		return nil, errors.New("invalid assignment state filter")
	}
	if input.Assignee != "" {
		if _, err := participantRecord(ctx, reader, input.TaskID, input.Assignee); err != nil {
			return nil, err
		}
	}
	limit, err := pageLimit(input.Limit, 50, 200)
	if err != nil {
		return nil, err
	}
	rows, err := reader.QueryContext(ctx, "SELECT "+assignmentColumns+" FROM assignments WHERE task_id=? AND id>? AND (?='' OR assignee_id=?) AND (?='' OR state=?) ORDER BY id LIMIT ?", input.TaskID, input.After, input.Assignee, input.Assignee, input.State, input.State, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Assignment{}
	for rows.Next() {
		assignment, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, assignment)
	}
	return result, rows.Err()
}

func (s *Store) Assignments(ctx context.Context, input ListAssignmentsInput) ([]Assignment, error) {
	return assignments(ctx, s.db, input)
}

// State transitions only express assignment bookkeeping. They do not decide a
// workflow's next step or authorize implementation, approvals, or process control.
func validAssignmentTransition(from, to string) bool {
	switch from {
	case "offered":
		return oneOf(to, "accepted", "declined", "cancelled")
	case "accepted":
		return oneOf(to, "working", "blocked", "needs_input", "submitted", "cancelled")
	case "working", "blocked", "needs_input":
		return oneOf(to, "working", "blocked", "needs_input", "submitted", "cancelled")
	default:
		return false
	}
}

func (s *Store) UpdateAssignment(ctx context.Context, input UpdateAssignmentInput) (Assignment, error) {
	if err := required("actor_participant_id", input.Actor, "summary", input.Summary); err != nil {
		return Assignment{}, err
	}
	if !oneOf(input.State, "accepted", "declined", "working", "blocked", "needs_input", "cancelled") {
		return Assignment{}, errors.New("invalid assignment update state; submit_result is required for results")
	}
	return coordinationMutation(ctx, s, input.TaskID, "participant:"+input.Actor, input.RequestKey, "update_assignment", input, func(tx *sql.Tx) (Assignment, error) {
		assignment, err := assignmentRecord(ctx, tx, input.TaskID, input.AssignmentID)
		if err != nil {
			return assignment, err
		}
		actor, err := activeParticipant(ctx, tx, input.TaskID, input.Actor)
		if err != nil {
			return assignment, err
		}
		task, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", input.TaskID))
		if err != nil {
			return assignment, err
		}
		if (input.State == "cancelled" && actor.RoutingIdentity != task.Owner) || (input.State != "cancelled" && actor.ID != assignment.Assignee) {
			return assignment, errors.New("only the declared task owner can cancel; only the declared assignee can report work")
		}
		if input.ExpectedVersion != assignment.Version {
			return assignment, errors.New("stale assignment version; read the current assignment before updating")
		}
		if !validAssignmentTransition(assignment.State, input.State) {
			return assignment, fmt.Errorf("invalid assignment transition from %s to %s", assignment.State, input.State)
		}
		if err := requireActiveTask(ctx, tx, input.TaskID, input.State == "cancelled"); err != nil {
			return assignment, err
		}
		_, err = tx.ExecContext(ctx, "UPDATE assignments SET state=?,summary=?,version=version+1,updated_at=? WHERE task_id=? AND id=?", input.State, input.Summary, now(), input.TaskID, input.AssignmentID)
		if err != nil {
			return assignment, err
		}
		assignment, err = assignmentRecord(ctx, tx, input.TaskID, input.AssignmentID)
		if err != nil {
			return assignment, err
		}
		recipientID := assignment.AssigningParticipant
		if input.State == "cancelled" {
			recipientID = assignment.Assignee
		}
		recipient, err := participantRecord(ctx, tx, input.TaskID, recipientID)
		if err != nil {
			return assignment, err
		}
		kind := "progress"
		if input.State == "accepted" || input.State == "declined" {
			kind = "answer"
		}
		err = publishAssignment(ctx, tx, assignment, actor.RoutingIdentity, recipient.RoutingIdentity, kind, "assignment_updated")
		return assignment, err
	})
}

func requireActiveTask(ctx context.Context, reader databaseReader, taskID string, cancelling bool) error {
	task, err := scanTask(reader.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", taskID))
	if err == nil && !cancelling && (task.Status == "complete" || task.Status == "cancelled") {
		err = errors.New("task is complete or cancelled; reconcile with its owner before continuing")
	}
	return err
}

func (s *Store) SubmitResult(ctx context.Context, input SubmitResultInput) (Assignment, error) {
	if err := required("actor_participant_id", input.Actor, "result.summary", input.Result.Summary, "result.next_action", input.Result.NextAction); err != nil {
		return Assignment{}, err
	}
	if !oneOf(input.Result.Outcome, "ready_for_review", "completed", "failed") {
		return Assignment{}, errors.New("result outcome must be ready_for_review, completed, or failed")
	}
	for _, check := range input.Result.Checks {
		if err := required("check.command", check.Command); err != nil {
			return Assignment{}, err
		}
	}
	return coordinationMutation(ctx, s, input.TaskID, "participant:"+input.Actor, input.RequestKey, "submit_result", input, func(tx *sql.Tx) (Assignment, error) {
		assignment, err := assignmentRecord(ctx, tx, input.TaskID, input.AssignmentID)
		if err != nil {
			return assignment, err
		}
		if input.Actor != assignment.Assignee {
			return assignment, errors.New("only the declared assignee can submit a result")
		}
		if input.ExpectedVersion != assignment.Version {
			return assignment, errors.New("stale assignment version; read the current assignment before submitting")
		}
		if !validAssignmentTransition(assignment.State, "submitted") {
			return assignment, fmt.Errorf("cannot submit a result from assignment state %s", assignment.State)
		}
		if err := requireActiveTask(ctx, tx, input.TaskID, false); err != nil {
			return assignment, err
		}
		result, err := json.Marshal(input.Result)
		if err != nil {
			return assignment, err
		}
		_, err = tx.ExecContext(ctx, "UPDATE assignments SET state='submitted',summary=?,result=?,version=version+1,updated_at=? WHERE task_id=? AND id=?", input.Result.Summary, string(result), now(), input.TaskID, input.AssignmentID)
		if err != nil {
			return assignment, err
		}
		assignment, err = assignmentRecord(ctx, tx, input.TaskID, input.AssignmentID)
		if err != nil {
			return assignment, err
		}
		actor, err := activeParticipant(ctx, tx, input.TaskID, assignment.Assignee)
		if err != nil {
			return assignment, err
		}
		recipient, err := participantRecord(ctx, tx, input.TaskID, assignment.AssigningParticipant)
		if err != nil {
			return assignment, err
		}
		err = publishAssignment(ctx, tx, assignment, actor.RoutingIdentity, recipient.RoutingIdentity, "result", "assignment_submitted")
		return assignment, err
	})
}

func publishAssignment(ctx context.Context, tx *sql.Tx, assignment Assignment, sender, recipient, messageKind, eventKind string) error {
	body, err := json.Marshal(assignment)
	if err != nil {
		return err
	}
	messageID := newID()
	_, err = tx.ExecContext(ctx, "INSERT INTO messages(id,task_id,sender,recipient,kind,text,created_at,assignment_id) VALUES(?,?,?,?,?,?,?,?)", messageID, assignment.TaskID, sender, recipient, messageKind, string(body), now(), assignment.ID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(map[string]any{"assignment_id": assignment.ID, "state": assignment.State, "version": assignment.Version, "assignee_id": assignment.Assignee, "message_id": messageID, "recipient": recipient, "result": assignment.Result})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO events(task_id,actor,kind,stage,attempt,message,data,created_at) VALUES(?,?,?,?,?,?,?,?)", assignment.TaskID, sender, eventKind, assignment.Stage, assignment.Attempt, assignment.Summary, string(data), now())
	return err
}
