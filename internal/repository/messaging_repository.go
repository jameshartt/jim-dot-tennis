package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jim-dot-tennis/internal/models"
)

// MessagingRepository handles database operations for messages and notifications
type MessagingRepository struct {
	db *sql.DB
}

// NewMessagingRepository creates a new repository for messaging operations
func NewMessagingRepository(db *sql.DB) *MessagingRepository {
	return &MessagingRepository{
		db: db,
	}
}

// CreateMessage creates a new message in the database
func (r *MessagingRepository) CreateMessage(ctx context.Context, message models.Message) (uint, error) {
	query := `
		INSERT INTO messages (
			title, content, sender_type, sender_id, category, importance,
			related_entity_type, related_entity_id, is_draft, sent_at, season_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	var id uint
	err := r.db.QueryRowContext(
		ctx,
		query,
		message.Title,
		message.Content,
		message.SenderType,
		message.SenderID,
		message.Category,
		message.Importance,
		message.RelatedEntityType,
		message.RelatedEntityID,
		message.IsDraft,
		message.SentAt,
		message.SeasonID,
	).Scan(&id)

	return id, err
}

// UpdateMessage updates an existing message
func (r *MessagingRepository) UpdateMessage(ctx context.Context, message models.Message) error {
	query := `
		UPDATE messages
		SET title = $1, 
		    content = $2, 
		    sender_type = $3, 
		    sender_id = $4, 
		    category = $5, 
		    importance = $6,
		    related_entity_type = $7, 
		    related_entity_id = $8, 
		    is_draft = $9, 
		    sent_at = $10, 
		    season_id = $11
		WHERE id = $12
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		message.Title,
		message.Content,
		message.SenderType,
		message.SenderID,
		message.Category,
		message.Importance,
		message.RelatedEntityType,
		message.RelatedEntityID,
		message.IsDraft,
		message.SentAt,
		message.SeasonID,
		message.ID,
	)

	return err
}

// GetMessageByID retrieves a message by its ID
func (r *MessagingRepository) GetMessageByID(ctx context.Context, id uint) (*models.Message, error) {
	query := `
		SELECT id, title, content, sender_type, sender_id, category, importance,
		       related_entity_type, related_entity_id, is_draft, sent_at, season_id,
		       created_at, updated_at
		FROM messages
		WHERE id = $1
	`

	var message models.Message
	var senderID, relatedEntityType sql.NullString
	var relatedEntityID, seasonID sql.NullInt64
	var sentAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&message.ID,
		&message.Title,
		&message.Content,
		&message.SenderType,
		&senderID,
		&message.Category,
		&message.Importance,
		&relatedEntityType,
		&relatedEntityID,
		&message.IsDraft,
		&sentAt,
		&seasonID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if senderID.Valid {
		message.SenderID = senderID.String
	}

	if relatedEntityType.Valid {
		message.RelatedEntityType = models.EntityType(relatedEntityType.String)
	}

	if relatedEntityID.Valid {
		message.RelatedEntityID = uint(relatedEntityID.Int64)
	}

	if sentAt.Valid {
		message.SentAt = &sentAt.Time
	}

	if seasonID.Valid {
		message.SeasonID = uint(seasonID.Int64)
	}

	// Get recipients for this message
	recipientsQuery := `
		SELECT id, message_id, recipient_type, recipient_id, is_read, read_at, created_at, updated_at
		FROM message_recipients
		WHERE message_id = $1
	`

	rows, err := r.db.QueryContext(ctx, recipientsQuery, id)
	if err != nil {
		return &message, err
	}
	defer rows.Close()

	for rows.Next() {
		var recipient models.MessageRecipient
		var recipientID sql.NullString
		var readAt sql.NullTime

		if err := rows.Scan(
			&recipient.ID,
			&recipient.MessageID,
			&recipient.RecipientType,
			&recipientID,
			&recipient.IsRead,
			&readAt,
			&recipient.CreatedAt,
			&recipient.UpdatedAt,
		); err != nil {
			return &message, err
		}

		if recipientID.Valid {
			recipient.RecipientID = recipientID.String
		}

		if readAt.Valid {
			recipient.ReadAt = &readAt.Time
		}

		message.Recipients = append(message.Recipients, recipient)
	}

	return &message, nil
}

// GetMessagesForRecipient gets all messages for a specific recipient
func (r *MessagingRepository) GetMessagesForRecipient(
	ctx context.Context,
	recipientType models.RecipientType,
	recipientID string,
	limit, offset int,
) ([]models.Message, error) {
	query := `
		SELECT m.id, m.title, m.content, m.sender_type, m.sender_id, m.category, m.importance,
		       m.related_entity_type, m.related_entity_id, m.is_draft, m.sent_at, m.season_id,
		       m.created_at, m.updated_at, 
		       mr.id as recipient_id, mr.is_read, mr.read_at
		FROM messages m
		JOIN message_recipients mr ON m.id = mr.message_id
		WHERE mr.recipient_type = $1 
		AND (mr.recipient_id = $2 OR mr.recipient_id IS NULL)
		AND m.is_draft = false
		AND m.sent_at IS NOT NULL
		ORDER BY m.sent_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.QueryContext(ctx, query, recipientType, recipientID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var message models.Message
		var recipient models.MessageRecipient
		var senderID, relatedEntityType sql.NullString
		var relatedEntityID, seasonID sql.NullInt64
		var sentAt, readAt sql.NullTime

		if err := rows.Scan(
			&message.ID,
			&message.Title,
			&message.Content,
			&message.SenderType,
			&senderID,
			&message.Category,
			&message.Importance,
			&relatedEntityType,
			&relatedEntityID,
			&message.IsDraft,
			&sentAt,
			&seasonID,
			&message.CreatedAt,
			&message.UpdatedAt,
			&recipient.ID,
			&recipient.IsRead,
			&readAt,
		); err != nil {
			return messages, err
		}

		if senderID.Valid {
			message.SenderID = senderID.String
		}

		if relatedEntityType.Valid {
			message.RelatedEntityType = models.EntityType(relatedEntityType.String)
		}

		if relatedEntityID.Valid {
			message.RelatedEntityID = uint(relatedEntityID.Int64)
		}

		if sentAt.Valid {
			message.SentAt = &sentAt.Time
		}

		if seasonID.Valid {
			message.SeasonID = uint(seasonID.Int64)
		}

		if readAt.Valid {
			recipient.ReadAt = &readAt.Time
		}

		recipient.MessageID = message.ID
		recipient.RecipientType = recipientType
		recipient.RecipientID = recipientID

		message.Recipients = []models.MessageRecipient{recipient}
		messages = append(messages, message)
	}

	return messages, nil
}

// AddMessageRecipients adds recipients to a message
func (r *MessagingRepository) AddMessageRecipients(
	ctx context.Context,
	messageID uint,
	recipients []models.MessageRecipient,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO message_recipients (message_id, recipient_type, recipient_id)
		VALUES ($1, $2, $3)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, recipient := range recipients {
		var recipientID interface{} = nil
		if recipient.RecipientID != "" {
			recipientID = recipient.RecipientID
		}

		_, err := stmt.ExecContext(ctx, messageID, recipient.RecipientType, recipientID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// MarkMessageAsRead marks a message as read for a specific recipient
func (r *MessagingRepository) MarkMessageAsRead(
	ctx context.Context,
	messageID uint,
	recipientType models.RecipientType,
	recipientID string,
) error {
	query := `
		UPDATE message_recipients
		SET is_read = true, read_at = CURRENT_TIMESTAMP
		WHERE message_id = $1 AND recipient_type = $2 AND recipient_id = $3
	`

	_, err := r.db.ExecContext(ctx, query, messageID, recipientType, recipientID)
	return err
}

// CreateNotification creates a new notification in the database
func (r *MessagingRepository) CreateNotification(ctx context.Context, notification models.Notification) (uint, error) {
	query := `
		INSERT INTO notifications (
			title, content, recipient_id, recipient_type, message_id, action_type,
			action_url, is_read, expires_at, importance, related_entity_type, related_entity_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	var id uint
	err := r.db.QueryRowContext(
		ctx,
		query,
		notification.Title,
		notification.Content,
		notification.RecipientID,
		notification.RecipientType,
		notification.MessageID,
		notification.ActionType,
		notification.ActionURL,
		notification.IsRead,
		notification.ExpiresAt,
		notification.Importance,
		notification.RelatedEntityType,
		notification.RelatedEntityID,
	).Scan(&id)

	return id, err
}

// GetActiveNotificationsForRecipient gets active notifications for a recipient
func (r *MessagingRepository) GetActiveNotificationsForRecipient(
	ctx context.Context,
	recipientType models.RecipientType,
	recipientID string,
) ([]models.Notification, error) {
	query := `
		SELECT id, title, content, recipient_id, recipient_type, message_id, action_type,
		       action_url, is_read, read_at, expires_at, importance, related_entity_type, 
		       related_entity_id, created_at, updated_at
		FROM notifications
		WHERE recipient_type = $1 
		AND recipient_id = $2
		AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, recipientType, recipientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var notification models.Notification
		var messageID sql.NullInt64
		var actionType, relatedEntityType, actionURL sql.NullString
		var readAt, expiresAt sql.NullTime
		var relatedEntityID sql.NullInt64

		if err := rows.Scan(
			&notification.ID,
			&notification.Title,
			&notification.Content,
			&notification.RecipientID,
			&notification.RecipientType,
			&messageID,
			&actionType,
			&actionURL,
			&notification.IsRead,
			&readAt,
			&expiresAt,
			&notification.Importance,
			&relatedEntityType,
			&relatedEntityID,
			&notification.CreatedAt,
			&notification.UpdatedAt,
		); err != nil {
			return notifications, err
		}

		if messageID.Valid {
			id := uint(messageID.Int64)
			notification.MessageID = &id
		}

		if actionType.Valid {
			notification.ActionType = models.ActionType(actionType.String)
		}

		if actionURL.Valid {
			notification.ActionURL = actionURL.String
		}

		if readAt.Valid {
			notification.ReadAt = &readAt.Time
		}

		if expiresAt.Valid {
			notification.ExpiresAt = &expiresAt.Time
		}

		if relatedEntityType.Valid {
			notification.RelatedEntityType = models.EntityType(relatedEntityType.String)
		}

		if relatedEntityID.Valid {
			notification.RelatedEntityID = uint(relatedEntityID.Int64)
		}

		notifications = append(notifications, notification)
	}

	return notifications, nil
}

// MarkNotificationAsRead marks a notification as read
func (r *MessagingRepository) MarkNotificationAsRead(ctx context.Context, notificationID uint) error {
	query := `
		UPDATE notifications
		SET is_read = true, read_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, notificationID)
	return err
}

// DeleteExpiredNotifications deletes all expired notifications
func (r *MessagingRepository) DeleteExpiredNotifications(ctx context.Context) (int, error) {
	query := `
		DELETE FROM notifications
		WHERE expires_at < CURRENT_TIMESTAMP
		RETURNING id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	return count, nil
}

// CreateScheduledMessage schedules a message for future delivery
func (r *MessagingRepository) CreateScheduledMessage(
	ctx context.Context, 
	scheduled models.ScheduledMessage,
) (uint, error) {
	query := `
		INSERT INTO scheduled_messages (message_id, scheduled_time, recurrence)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	var id uint
	err := r.db.QueryRowContext(
		ctx,
		query,
		scheduled.MessageID,
		scheduled.ScheduledTime,
		scheduled.Recurrence,
	).Scan(&id)

	return id, err
}

// GetDueScheduledMessages gets all scheduled messages that are due to be sent
func (r *MessagingRepository) GetDueScheduledMessages(ctx context.Context) ([]models.ScheduledMessage, error) {
	query := `
		SELECT sm.id, sm.message_id, sm.scheduled_time, sm.recurrence, sm.is_sent, 
		       sm.last_sent_at, sm.created_at, sm.updated_at,
		       m.title, m.content
		FROM scheduled_messages sm
		JOIN messages m ON sm.message_id = m.id
		WHERE sm.scheduled_time <= CURRENT_TIMESTAMP
		AND (sm.is_sent = false OR sm.recurrence != 'None')
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.ScheduledMessage
	for rows.Next() {
		var message models.ScheduledMessage
		var messageDetails models.Message
		var lastSentAt sql.NullTime

		if err := rows.Scan(
			&message.ID,
			&message.MessageID,
			&message.ScheduledTime,
			&message.Recurrence,
			&message.IsSent,
			&lastSentAt,
			&message.CreatedAt,
			&message.UpdatedAt,
			&messageDetails.Title,
			&messageDetails.Content,
		); err != nil {
			return messages, err
		}

		if lastSentAt.Valid {
			message.LastSentAt = &lastSentAt.Time
		}

		messageDetails.ID = message.MessageID
		message.Message = &messageDetails

		messages = append(messages, message)
	}

	return messages, nil
}

// UpdateScheduledMessageAfterSending updates a scheduled message after it's sent
func (r *MessagingRepository) UpdateScheduledMessageAfterSending(
	ctx context.Context, 
	id uint, 
	nextScheduledTime *time.Time,
) error {
	query := `
		UPDATE scheduled_messages
		SET is_sent = true, 
		    last_sent_at = CURRENT_TIMESTAMP
	`

	args := []interface{}{id}
	argIndex := 2

	if nextScheduledTime != nil {
		query += fmt.Sprintf(", scheduled_time = $%d", argIndex)
		args = append(args, nextScheduledTime)
		argIndex++
	}

	query += fmt.Sprintf(" WHERE id = $1")

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// SaveDeliveryPreferences saves a player's delivery preferences
func (r *MessagingRepository) SaveDeliveryPreferences(
	ctx context.Context, 
	prefs models.DeliveryPreference,
) (uint, error) {
	query := `
		INSERT INTO delivery_preferences (
			player_id, email_enabled, push_enabled, sms_enabled, in_app_enabled,
			availability_reminders, selection_notifications, fixture_reminders, result_notifications
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (player_id) DO UPDATE
		SET email_enabled = $2, 
		    push_enabled = $3, 
		    sms_enabled = $4, 
		    in_app_enabled = $5,
		    availability_reminders = $6, 
		    selection_notifications = $7, 
		    fixture_reminders = $8, 
		    result_notifications = $9
		RETURNING id
	`

	var id uint
	err := r.db.QueryRowContext(
		ctx,
		query,
		prefs.PlayerID,
		prefs.EmailEnabled,
		prefs.PushEnabled,
		prefs.SMSEnabled,
		prefs.InAppEnabled,
		prefs.AvailabilityReminders,
		prefs.SelectionNotifications,
		prefs.FixtureReminders,
		prefs.ResultNotifications,
	).Scan(&id)

	return id, err
}

// GetDeliveryPreferences gets a player's delivery preferences
func (r *MessagingRepository) GetDeliveryPreferences(
	ctx context.Context, 
	playerID string,
) (*models.DeliveryPreference, error) {
	query := `
		SELECT id, player_id, email_enabled, push_enabled, sms_enabled, in_app_enabled,
		       availability_reminders, selection_notifications, fixture_reminders, 
		       result_notifications, created_at, updated_at
		FROM delivery_preferences
		WHERE player_id = $1
	`

	var prefs models.DeliveryPreference
	err := r.db.QueryRowContext(ctx, query, playerID).Scan(
		&prefs.ID,
		&prefs.PlayerID,
		&prefs.EmailEnabled,
		&prefs.PushEnabled,
		&prefs.SMSEnabled,
		&prefs.InAppEnabled,
		&prefs.AvailabilityReminders,
		&prefs.SelectionNotifications,
		&prefs.FixtureReminders,
		&prefs.ResultNotifications,
		&prefs.CreatedAt,
		&prefs.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create default preferences
		prefs = models.DeliveryPreference{
			PlayerID:               playerID,
			EmailEnabled:           true,
			PushEnabled:            false,
			SMSEnabled:             false,
			InAppEnabled:           true,
			AvailabilityReminders:  true,
			SelectionNotifications: true,
			FixtureReminders:       true,
			ResultNotifications:    true,
		}
		return &prefs, nil
	}

	if err != nil {
		return nil, err
	}

	return &prefs, nil
} 