// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package webpush

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/SherClockHolmes/webpush-go"

	"jim-dot-tennis/internal/database"
)

// Subscription represents a web push subscription
type Subscription struct {
	ID          int64     `db:"id" json:"-"`
	Endpoint    string    `db:"endpoint" json:"endpoint"`
	P256dh      string    `db:"p256dh" json:"p256dh"`
	Auth        string    `db:"auth" json:"auth"`
	Platform    string    `db:"platform" json:"platform,omitempty"`
	UserAgent   string    `db:"user_agent" json:"userAgent,omitempty"`
	PlayerToken *string   `db:"player_token" json:"playerToken,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt,omitempty"`
}

// SubscriptionRequest represents the incoming subscription data from the browser
type SubscriptionRequest struct {
	Endpoint    string `json:"endpoint"`
	PlayerToken string `json:"playerToken"`
	Keys        struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Service manages web push operations
type Service struct {
	db *database.DB
}

// New creates a new WebPush service
func New(db *database.DB) *Service {
	return &Service{db: db}
}

// GenerateVAPIDKeys generates a new pair of VAPID keys if none exist
func (s *Service) GenerateVAPIDKeys() (publicKey, privateKey string, err error) {
	// Check if keys already exist
	var keys struct {
		PublicKey  string `db:"public_key"`
		PrivateKey string `db:"private_key"`
	}

	err = s.db.QueryRow("SELECT public_key, private_key FROM vapid_keys LIMIT 1").Scan(&keys.PublicKey, &keys.PrivateKey)
	if err == nil {
		// Keys already exist, verify format
		if !isValidVAPIDKey(keys.PublicKey) {
			log.Printf("Existing VAPID public key is not in the correct format, generating new keys...")
		} else {
			return keys.PublicKey, keys.PrivateKey, nil
		}
	}

	if err != nil && err != sql.ErrNoRows {
		return "", "", err
	}

	// Generate new VAPID keys
	vapidPrivate, vapidPublic, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate VAPID keys: %v", err)
	}

	// Ensure the public key is in the correct format
	if !isValidVAPIDKey(vapidPublic) {
		return "", "", fmt.Errorf("generated VAPID public key is not in the correct format")
	}

	// Store the keys in the database
	_, err = s.db.Exec(
		"INSERT INTO vapid_keys (public_key, private_key) VALUES ($1, $2)",
		vapidPublic, vapidPrivate,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to store VAPID keys: %v", err)
	}

	return vapidPublic, vapidPrivate, nil
}

// isValidVAPIDKey checks if a VAPID key is in the correct format
func isValidVAPIDKey(key string) bool {
	// VAPID public key should be a base64 URL-safe string
	// It should decode to 65 bytes (uncompressed public key)
	// and should start with "BP" when decoded (indicating it's a valid ECDSA P-256 public key)

	// Add padding if needed
	padding := (4 - len(key)%4) % 4
	key = key + strings.Repeat("=", padding)

	// Replace URL-safe characters
	key = strings.ReplaceAll(key, "-", "+")
	key = strings.ReplaceAll(key, "_", "/")

	// Decode the key
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return false
	}

	// Check length (should be 65 bytes for uncompressed public key)
	if len(decoded) != 65 {
		return false
	}

	// Check if it starts with 0x04 (uncompressed public key)
	if decoded[0] != 0x04 {
		return false
	}

	return true
}

// GetVAPIDKeys returns the stored VAPID keys
func (s *Service) GetVAPIDKeys() (publicKey, privateKey string, err error) {
	log.Printf("Attempting to retrieve VAPID keys from database...")

	var keys struct {
		PublicKey  string `db:"public_key"`
		PrivateKey string `db:"private_key"`
	}

	err = s.db.QueryRow("SELECT public_key, private_key FROM vapid_keys LIMIT 1").Scan(&keys.PublicKey, &keys.PrivateKey)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No VAPID keys found in database")
			return "", "", errors.New("no VAPID keys found")
		}
		log.Printf("Database error retrieving VAPID keys: %v", err)
		return "", "", err
	}

	// Verify the public key format
	if !isValidVAPIDKey(keys.PublicKey) {
		log.Printf("Stored VAPID public key is not in the correct format, generating new keys...")
		return s.GenerateVAPIDKeys()
	}

	log.Printf("Successfully retrieved VAPID keys from database")
	return keys.PublicKey, keys.PrivateKey, nil
}

// SaveSubscription saves a push subscription to the database.
// If a subscription with the same endpoint already exists, it updates the player_token.
func (s *Service) SaveSubscription(sub *Subscription) error {
	// Upsert: update player_token if endpoint already exists
	_, err := s.db.Exec(
		`INSERT INTO push_subscriptions (endpoint, p256dh, auth, platform, user_agent, player_token)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT(endpoint) DO UPDATE SET player_token = $6, p256dh = $2, auth = $3`,
		sub.Endpoint, sub.P256dh, sub.Auth, sub.Platform, sub.UserAgent, sub.PlayerToken,
	)
	return err
}

// DeleteSubscription removes a subscription by endpoint
func (s *Service) DeleteSubscription(endpoint string) error {
	_, err := s.db.Exec(
		"DELETE FROM push_subscriptions WHERE endpoint = $1",
		endpoint,
	)
	return err
}

// GetAllSubscriptions retrieves all push subscriptions
func (s *Service) GetAllSubscriptions() ([]Subscription, error) {
	var subs []Subscription
	err := s.db.Select(&subs, "SELECT * FROM push_subscriptions")
	return subs, err
}

// HasSubscription returns true if the given player token has at least one active push subscription
func (s *Service) HasSubscription(playerToken string) bool {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM push_subscriptions WHERE player_token = $1", playerToken).Scan(&count)
	return err == nil && count > 0
}

// CleanupStaleSubscriptions removes subscriptions older than the given duration
func (s *Service) CleanupStaleSubscriptions(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	result, err := s.db.Exec("DELETE FROM push_subscriptions WHERE created_at < $1", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetSubscriptionsByPlayerToken returns all push subscriptions for a given player token
func (s *Service) GetSubscriptionsByPlayerToken(playerToken string) ([]Subscription, error) {
	var subs []Subscription
	err := s.db.Select(&subs, "SELECT * FROM push_subscriptions WHERE player_token = $1", playerToken)
	return subs, err
}

// SendToPlayer sends a push notification to all devices for a given player token.
// Returns the number of successful sends and any error.
func (s *Service) SendToPlayer(playerToken string, payload map[string]interface{}) (int, error) {
	subs, err := s.GetSubscriptionsByPlayerToken(playerToken)
	if err != nil {
		return 0, fmt.Errorf("failed to get subscriptions for player %s: %v", playerToken, err)
	}

	if len(subs) == 0 {
		return 0, nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal notification payload: %v", err)
	}

	successCount := 0
	for _, sub := range subs {
		if err := s.sendRawNotification(sub, payloadBytes); err != nil {
			log.Printf("Failed to send notification to player %s device %s: %v", playerToken, sub.Endpoint, err)
		} else {
			successCount++
		}
	}

	return successCount, nil
}

// SendNotification sends a push notification to a subscription with a simple message string
func (s *Service) SendNotification(sub Subscription, message string) error {
	payload, err := json.Marshal(map[string]string{
		"message": message,
	})
	if err != nil {
		return err
	}
	return s.sendRawNotification(sub, payload)
}

// sendRawNotification sends a pre-encoded payload to a push subscription
func (s *Service) sendRawNotification(sub Subscription, payload []byte) error {
	vapidPublic, vapidPrivate, err := s.GetVAPIDKeys()
	if err != nil {
		return err
	}

	log.Printf("Sending notification to endpoint: %s", sub.Endpoint)

	options := &webpush.Options{
		VAPIDPublicKey:  vapidPublic,
		VAPIDPrivateKey: vapidPrivate,
		TTL:             30,
		Subscriber:      "https://jim.tennis",
	}

	resp, err := webpush.SendNotification(
		payload,
		&webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		},
		options,
	)

	if err != nil {
		log.Printf("Error sending notification: %v", err)
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode >= 400 {
		log.Printf("Failed to send notification, status: %d", resp.StatusCode)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusUnauthorized {
			log.Printf("Removing stale subscription (status %d): %s", resp.StatusCode, sub.Endpoint)
			s.DeleteSubscription(sub.Endpoint)
		}
		return fmt.Errorf("failed to send notification, status: %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully sent notification")
	return nil
}

// SendToAll sends a push notification to all subscriptions
func (s *Service) SendToAll(message string) error {
	log.Printf("Starting SendToAll with message: %s", message)
	startTime := time.Now()

	subs, err := s.GetAllSubscriptions()
	if err != nil {
		log.Printf("Error retrieving subscriptions: %v", err)
		return fmt.Errorf("failed to get subscriptions: %v", err)
	}
	log.Printf("Retrieved %d subscriptions to process", len(subs))

	var successCount, failureCount int
	var lastErr error
	for i, sub := range subs {
		log.Printf("Processing subscription %d/%d (endpoint: %s)", i+1, len(subs), sub.Endpoint)
		sendStart := time.Now()

		if err := s.SendNotification(sub, message); err != nil {
			log.Printf("Failed to send to subscription %d/%d (endpoint: %s): %v",
				i+1, len(subs), sub.Endpoint, err)
			failureCount++
			lastErr = err
		} else {
			log.Printf("Successfully sent to subscription %d/%d (endpoint: %s) in %v",
				i+1, len(subs), sub.Endpoint, time.Since(sendStart))
			successCount++
		}
	}

	totalDuration := time.Since(startTime)
	log.Printf("SendToAll completed in %v", totalDuration)
	log.Printf("Results: %d successful, %d failed out of %d total subscriptions",
		successCount, failureCount, len(subs))

	if failureCount > 0 {
		return fmt.Errorf("some notifications failed (%d/%d): last error: %v",
			failureCount, len(subs), lastErr)
	}
	return nil
}

// ListVAPIDKeys returns all VAPID keys in the database (for debugging)
func (s *Service) ListVAPIDKeys() error {
	log.Printf("Listing all VAPID keys in database...")

	rows, err := s.db.Query("SELECT public_key, private_key FROM vapid_keys")
	if err != nil {
		return fmt.Errorf("error querying VAPID keys: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var publicKey, privateKey string
		if err := rows.Scan(&publicKey, &privateKey); err != nil {
			return fmt.Errorf("error scanning VAPID key row: %v", err)
		}
		count++
		log.Printf("VAPID Key #%d:", count)
		log.Printf("  Public Key:  %s", publicKey)
		log.Printf("  Private Key: %s", privateKey[:10]+"..."+privateKey[len(privateKey)-10:]) // Only show part of private key for security
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating VAPID key rows: %v", err)
	}

	log.Printf("Found %d VAPID key(s) in database", count)
	return nil
}

// ResetVAPIDKeys deletes existing VAPID keys and generates new ones
func (s *Service) ResetVAPIDKeys() (publicKey, privateKey string, err error) {
	log.Printf("Resetting VAPID keys...")

	// Delete existing keys
	_, err = s.db.Exec("DELETE FROM vapid_keys")
	if err != nil {
		return "", "", fmt.Errorf("failed to delete existing VAPID keys: %v", err)
	}

	// Generate new keys
	return s.GenerateVAPIDKeys()
}
