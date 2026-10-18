package comms

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) RegisterParticipant(ctx context.Context, input RegisterParticipantInput) (Participant, error) {
	if err := required("instance_id", input.InstanceID, "role", input.Role, "routing_identity", input.RoutingIdentity); err != nil {
		return Participant{}, err
	}
	return coordinationMutation(ctx, s, input.TaskID, "instance:"+input.InstanceID, input.RequestKey, "register_participant", input, func(tx *sql.Tx) (Participant, error) {
		if input.ReplacesParticipantID != "" {
			previous, err := activeParticipant(ctx, tx, input.TaskID, input.ReplacesParticipantID)
			if err != nil {
				return Participant{}, err
			}
			if previous.Role != input.Role || previous.RoutingIdentity != input.RoutingIdentity || previous.InstanceID == input.InstanceID {
				return Participant{}, errors.New("replacement must use a new instance ID with the same role and routing identity")
			}
			if _, err := tx.ExecContext(ctx, "UPDATE participants SET retired_at=? WHERE task_id=? AND id=?", now(), input.TaskID, previous.ID); err != nil {
				return Participant{}, err
			}
		}

		var used int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM participants WHERE task_id=? AND (instance_id=? OR (routing_identity=? AND retired_at IS NULL))", input.TaskID, input.InstanceID, input.RoutingIdentity).Scan(&used); err != nil {
			return Participant{}, err
		}
		if used != 0 {
			return Participant{}, errors.New("instance or routing identity is already registered in this task; reuse its original registration request or select a distinct routing identity")
		}
		participant := Participant{ID: newID(), TaskID: input.TaskID, InstanceID: input.InstanceID, Role: input.Role, RoutingIdentity: input.RoutingIdentity, SessionRef: input.SessionRef, RegisteredAt: now()}
		_, err := tx.ExecContext(ctx, "INSERT INTO participants("+participantColumns+") VALUES(?,?,?,?,?,?,?,NULL)", participant.ID, participant.TaskID, participant.InstanceID, participant.Role, participant.RoutingIdentity, participant.SessionRef, participant.RegisteredAt)
		if err != nil {
			return participant, err
		}
		err = appendEvent(ctx, tx, input.TaskID, input.RoutingIdentity, "participant_registered", "Participant registered; availability is unknown", map[string]any{"participant_id": participant.ID, "instance_id": participant.InstanceID, "role": participant.Role, "routing_identity": participant.RoutingIdentity, "replaces_participant_id": input.ReplacesParticipantID})
		return participant, err
	})
}

func participants(ctx context.Context, reader databaseReader, input ListParticipantsInput) ([]Participant, error) {
	if _, err := scanTask(reader.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", input.TaskID)); err != nil {
		return nil, err
	}
	limit, err := pageLimit(input.Limit, 50, 200)
	if err != nil {
		return nil, err
	}
	rows, err := reader.QueryContext(ctx, "SELECT "+participantColumns+" FROM participants WHERE task_id=? AND id>? ORDER BY id LIMIT ?", input.TaskID, input.After, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Participant{}
	for rows.Next() {
		participant, err := scanParticipant(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, participant)
	}
	return result, rows.Err()
}

func (s *Store) Participants(ctx context.Context, input ListParticipantsInput) ([]Participant, error) {
	return participants(ctx, s.db, input)
}
