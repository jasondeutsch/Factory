package comms

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type databaseReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const participantColumns = "id,task_id,instance_id,role,routing_identity,session_ref,registered_at,retired_at"
const assignmentColumns = "id,task_id,assigning_participant_id,assignee_id,title,instructions,inputs,expected_output,stage,attempt,state,summary,version,created_at,updated_at,result,handoff"

func scanParticipant(row scanner) (Participant, error) {
	var participant Participant
	err := row.Scan(&participant.ID, &participant.TaskID, &participant.InstanceID, &participant.Role, &participant.RoutingIdentity, &participant.SessionRef, &participant.RegisteredAt, &participant.RetiredAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = errors.New("participant not found in this task")
	}
	return participant, err
}

func participantRecord(ctx context.Context, reader databaseReader, taskID, id string) (Participant, error) {
	return scanParticipant(reader.QueryRowContext(ctx, "SELECT "+participantColumns+" FROM participants WHERE task_id=? AND id=?", taskID, id))
}

func activeParticipant(ctx context.Context, reader databaseReader, taskID, id string) (Participant, error) {
	participant, err := participantRecord(ctx, reader, taskID, id)
	if err == nil && participant.RetiredAt != nil {
		err = errors.New("participant instance is retired; the orchestrator must explicitly replace its assignments")
	}
	return participant, err
}

func scanAssignment(row scanner) (Assignment, error) {
	var assignment Assignment
	var inputs, expected, handoff string
	var result sql.NullString
	err := row.Scan(&assignment.ID, &assignment.TaskID, &assignment.AssigningParticipant, &assignment.Assignee, &assignment.Title, &assignment.Instructions, &inputs, &expected, &assignment.Stage, &assignment.Attempt, &assignment.State, &assignment.Summary, &assignment.Version, &assignment.CreatedAt, &assignment.UpdatedAt, &result, &handoff)
	if errors.Is(err, sql.ErrNoRows) {
		return assignment, errors.New("assignment not found in this task")
	}
	if err != nil {
		return assignment, err
	}
	if err := json.Unmarshal([]byte(inputs), &assignment.Inputs); err != nil {
		return assignment, err
	}
	if err := json.Unmarshal([]byte(expected), &assignment.ExpectedOutput); err != nil {
		return assignment, err
	}
	if err := json.Unmarshal([]byte(handoff), &assignment.Handoff); err != nil {
		return assignment, err
	}
	if result.Valid {
		assignment.Result = new(AssignmentResult)
		if err := json.Unmarshal([]byte(result.String), assignment.Result); err != nil {
			return assignment, err
		}
	}
	return assignment, nil
}

func assignmentRecord(ctx context.Context, reader databaseReader, taskID, id string) (Assignment, error) {
	return scanAssignment(reader.QueryRowContext(ctx, "SELECT "+assignmentColumns+" FROM assignments WHERE task_id=? AND id=?", taskID, id))
}

// coordinationMutation replays the original response before checking the current
// version. A lost successful response can therefore be retried without producing
// duplicate state changes, inbox messages, or events. Different payloads conflict.
func coordinationMutation[Input, Output any](ctx context.Context, store *Store, taskID, actorKey, requestKey, operation string, input Input, change func(*sql.Tx) (Output, error)) (Output, error) {
	var output Output
	if err := required("task_id", taskID, "request_key", requestKey); err != nil {
		return output, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return output, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return output, err
	}
	defer tx.Rollback()
	var oldOperation, oldInput, oldOutput string
	err = tx.QueryRowContext(ctx, "SELECT operation,input,output FROM coordination_requests WHERE task_id=? AND actor_key=? AND request_key=?", taskID, actorKey, requestKey).Scan(&oldOperation, &oldInput, &oldOutput)
	if err == nil {
		if oldOperation != operation || oldInput != string(encoded) {
			return output, errors.New("request_key was already used for a different coordination request")
		}
		err = json.Unmarshal([]byte(oldOutput), &output)
		return output, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return output, err
	}
	if _, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", taskID)); err != nil {
		return output, err
	}
	output, err = change(tx)
	if err != nil {
		return output, err
	}
	response, err := json.Marshal(output)
	if err != nil {
		return output, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO coordination_requests(task_id,actor_key,request_key,operation,input,output) VALUES(?,?,?,?,?,?)", taskID, actorKey, requestKey, operation, string(encoded), string(response))
	if err != nil {
		return output, err
	}
	return output, tx.Commit()
}
