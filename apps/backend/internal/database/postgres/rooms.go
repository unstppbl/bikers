package postgres

import (
	"context"
	"errors"

	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

func readRoomAccess(ctx context.Context, q querier, rideID, userID string) (models.RoomAccess, error) {
	var access models.RoomAccess
	err := q.QueryRow(ctx, `SELECT p.state,u.status FROM participation p JOIN users u ON u.id=p.user_id WHERE p.ride_id=$1 AND p.user_id=$2`, rideID, userID).Scan(&access.ParticipationState, &access.UserStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.RoomAccess{}, nil
	}
	return access, repositoryError(err)
}
func (s *Store) RoomAccess(ctx context.Context, rideID, userID string) (models.RoomAccess, error) {
	return readRoomAccess(ctx, s.pool, rideID, userID)
}
func (t *rideTransaction) RoomAccess(userID string) (models.RoomAccess, error) {
	return readRoomAccess(t.ctx, t.tx, t.ride.ID, userID)
}

func (s *Store) Messages(ctx context.Context, rideID, userID string, after int64, limit int) (models.MessagePage, error) {
	rows, err := s.pool.Query(ctx, `SELECT m.id::text,m.sequence,m.sender_id::text,u.display_name,m.body,m.created_at,m.client_message_id::text FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.ride_id=$1 AND m.sequence>$2 AND EXISTS(SELECT 1 FROM participation vp JOIN users vu ON vu.id=vp.user_id WHERE vp.ride_id=m.ride_id AND vp.user_id::text=$4 AND vp.state='approved' AND vu.status='active') AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='message' AND h.target_id=m.id) AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id::text=$4 AND b.blocked_id=m.sender_id) OR (b.blocked_id::text=$4 AND b.blocker_id=m.sender_id)) ORDER BY m.sequence LIMIT $3`, rideID, after, limit, userID)
	if err != nil {
		return models.MessagePage{}, repositoryError(err)
	}
	defer rows.Close()
	page := models.MessagePage{Messages: []models.Message{}, NextSequence: after}
	for rows.Next() {
		var message models.Message
		if err = rows.Scan(&message.ID, &message.Sequence, &message.SenderID, &message.SenderName, &message.Body, &message.CreatedAt, &message.ClientMessageID); err != nil {
			return models.MessagePage{}, repositoryError(err)
		}
		page.Messages = append(page.Messages, message)
		page.NextSequence = message.Sequence
	}
	if err = rows.Err(); err != nil {
		return models.MessagePage{}, repositoryError(err)
	}
	return page, nil
}
func (t *rideTransaction) Message(senderID, clientMessageID string) (models.Message, error) {
	var message models.Message
	err := t.tx.QueryRow(t.ctx, `SELECT m.id::text,m.sequence,m.sender_id::text,u.display_name,m.body,m.created_at,m.client_message_id::text FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.ride_id=$1 AND m.sender_id=$2 AND m.client_message_id=$3`, t.ride.ID, senderID, clientMessageID).Scan(&message.ID, &message.Sequence, &message.SenderID, &message.SenderName, &message.Body, &message.CreatedAt, &message.ClientMessageID)
	return message, repositoryError(err)
}
func (t *rideTransaction) AppendMessage(message models.Message) (models.Message, error) {
	// WithRide already owns this row lock through commit. Sequence assignment
	// therefore follows commit visibility, so reconnect cursors cannot skip rows.
	err := t.tx.QueryRow(t.ctx, "UPDATE rides SET message_sequence=message_sequence+1 WHERE id=$1 RETURNING message_sequence", t.ride.ID).Scan(&message.Sequence)
	if err != nil {
		return models.Message{}, repositoryError(err)
	}
	_, err = t.tx.Exec(t.ctx, `INSERT INTO messages(id,ride_id,sender_id,sequence,client_message_id,body,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, message.ID, t.ride.ID, message.SenderID, message.Sequence, message.ClientMessageID, message.Body, message.CreatedAt)
	if err != nil {
		return models.Message{}, repositoryError(err)
	}
	return message, nil
}
