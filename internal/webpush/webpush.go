package webpush

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
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
		// Keys already exist
		return keys.PublicKey, keys.PrivateKey, nil
	}
	
	if err != sql.ErrNoRows {
		return "", "", err
	}
	
	// Generate new VAPID keys
	vapidPrivate, vapidPublic, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate VAPID keys: %v", err)
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
	
	log.Printf("Successfully retrieved VAPID keys from database")
	log.Printf("Public key length: %d", len(keys.PublicKey))
	log.Printf("Private key length: %d", len(keys.PrivateKey))
	
	return keys.PublicKey, keys.PrivateKey, nil
}

// SaveSubscription saves a push subscription to the database
func (s *Service) SaveSubscription(sub *Subscription) error {
	_, err := s.db.Exec(
		"INSERT INTO push_subscriptions (endpoint, p256dh, auth, user_agent) VALUES ($1, $2, $3, $4)",
		sub.Endpoint, sub.P256dh, sub.Auth, sub.UserAgent,
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
	
	// Create the message payload
	payload, err := json.Marshal(map[string]string{
		"message": message,
	})
	if err != nil {
		return err
	}
	
	// Send the notification
	resp, err := webpush.SendNotification(
		payload,
		&webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		},
		&webpush.Options{
			VAPIDPublicKey:  vapidPublic,
			VAPIDPrivateKey: vapidPrivate,
			TTL:             30,
			Subscriber:      "mailto:admin@jim.tennis",
		},
	)
	
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		// If the subscription is invalid (404) or expired (410), remove it
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			s.DeleteSubscription(sub.Endpoint)
		}
		return fmt.Errorf("failed to send notification, status: %d", resp.StatusCode)
	}
	
	return nil
}

// SendToAll sends a push notification to all subscriptions
func (s *Service) SendToAll(message string) error {
	subs, err := s.GetAllSubscriptions()
	if err != nil {
		return err
	}
	
	var lastErr error
	for _, sub := range subs {
		err := s.SendNotification(sub, message)
		if err != nil {
			lastErr = err
		}
	}
	
	return lastErr
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