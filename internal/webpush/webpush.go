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
	ID        int64     `db:"id" json:"-"`
	Endpoint  string    `db:"endpoint" json:"endpoint"`
	P256dh    string    `db:"p256dh" json:"p256dh"`
	Auth      string    `db:"auth" json:"auth"`
	Platform  string    `db:"platform" json:"platform,omitempty"`
	UserAgent string    `db:"user_agent" json:"userAgent,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt,omitempty"`
}

// SubscriptionRequest represents the incoming subscription data from the browser
type SubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
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

// SaveSubscription saves a push subscription to the database
func (s *Service) SaveSubscription(sub *Subscription) error {
	_, err := s.db.Exec(
		"INSERT INTO push_subscriptions (endpoint, p256dh, auth, platform, user_agent) VALUES ($1, $2, $3, $4, $5)",
		sub.Endpoint, sub.P256dh, sub.Auth, sub.Platform, sub.UserAgent,
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

// SendNotification sends a push notification to a subscription
func (s *Service) SendNotification(sub Subscription, message string) error {
	vapidPublic, vapidPrivate, err := s.GetVAPIDKeys()
	if err != nil {
		return err
	}

	// Create standard payload
	payload, err := json.Marshal(map[string]string{
		"message": message,
	})
	if err != nil {
		return err
	}

	// Log notification attempt
	log.Printf("Sending notification to endpoint: %s", sub.Endpoint)

	// Create webpush options
	options := &webpush.Options{
		VAPIDPublicKey:  vapidPublic,
		VAPIDPrivateKey: vapidPrivate,
		TTL:             30,
		Subscriber:      "https://jim.tennis", // Use website URL instead of mailto:
	}

	// Send the notification using the webpush-go library
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

	// Read and log the response body for debugging
	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode >= 400 {
		log.Printf("Failed to send notification, status: %d", resp.StatusCode)
		// If the subscription is invalid (404) or expired (410), remove it
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
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
		log.Printf("  Private Key: %s", privateKey[:10] + "..." + privateKey[len(privateKey)-10:]) // Only show part of private key for security
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