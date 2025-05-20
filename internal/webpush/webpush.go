package webpush

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"jim-dot-tennis/internal/database"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"bytes"
)

// Subscription represents a web push subscription
type Subscription struct {
	ID        int64     `db:"id" json:"-"`
	Endpoint  string    `db:"endpoint" json:"endpoint"`
	P256dh    string    `db:"p256dh" json:"p256dh"`
	Auth      string    `db:"auth" json:"auth"`
	UserAgent string    `db:"user_agent" json:"userAgent,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt,omitempty"`
	Platform  string    `db:"platform" json:"platform,omitempty"` // "web", "safari", or "chrome"
}

// SubscriptionRequest represents the incoming subscription data from the browser
type SubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// SafariNotificationPayload represents the payload format required by Safari
type SafariNotificationPayload struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Icon     string `json:"icon"`
	Badge    string `json:"badge"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Actions  []map[string]string    `json:"actions,omitempty"`
	Tag      string                 `json:"tag,omitempty"`
	Renotify bool                   `json:"renotify,omitempty"`
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
	log.Printf("Public key length: %d", len(keys.PublicKey))
	log.Printf("Private key length: %d", len(keys.PrivateKey))
	
	return keys.PublicKey, keys.PrivateKey, nil
}

// isSafariSubscription checks if a subscription is from Safari
func isSafariSubscription(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "safari") && 
	       !strings.Contains(strings.ToLower(userAgent), "chrome")
}

// getPlatform determines the platform from the user agent
func getPlatform(userAgent string) string {
	ua := strings.ToLower(userAgent)
	if strings.Contains(ua, "chrome") {
		return "chrome"
	} else if strings.Contains(ua, "safari") {
		return "safari"
	}
	return "web"
}

// SaveSubscription saves a push subscription to the database
func (s *Service) SaveSubscription(sub *Subscription) error {
	// Determine platform
	sub.Platform = getPlatform(sub.UserAgent)
	
	// Log subscription details for debugging
	log.Printf("Saving subscription for platform: %s", sub.Platform)
	log.Printf("User Agent: %s", sub.UserAgent)
	log.Printf("Endpoint: %s", sub.Endpoint)
	
	_, err := s.db.Exec(
		"INSERT INTO push_subscriptions (endpoint, p256dh, auth, user_agent, platform) VALUES ($1, $2, $3, $4, $5)",
		sub.Endpoint, sub.P256dh, sub.Auth, sub.UserAgent, sub.Platform,
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

// SafariAuthHeaders returns the required headers for Safari push notifications
func getSafariAuthHeaders(vapidPublic, vapidPrivate string) (map[string]string, error) {
	// Safari requires a specific JWT token format for authentication
	// We'll use the VAPID keys to create this token
	now := time.Now().Unix()
	exp := now + 12*3600 // 12 hours expiration

	// Create the JWT header
	header := map[string]string{
		"typ": "JWT",
		"alg": "ES256",
	}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JWT header: %v", err)
	}

	// Create the JWT claims
	claims := map[string]interface{}{
		"iss": "https://jim.tennis",
		"sub": "mailto:admin@jim.tennis",
		"aud": "https://web.push.apple.com",
		"iat": now,
		"exp": exp,
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JWT claims: %v", err)
	}

	// Encode header and claims
	headerB64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := headerB64 + "." + claimsB64

	// Sign the JWT using the VAPID private key
	// Note: This is a simplified version. In production, use a proper JWT library
	signature, err := signJWT(signingInput, vapidPrivate)
	if err != nil {
		return nil, fmt.Errorf("failed to sign JWT: %v", err)
	}

	// Combine to form the JWT
	jwt := signingInput + "." + signature

	return map[string]string{
		"Authorization": "Bearer " + jwt,
		"apns-push-type": "web",
		"apns-expiration": "0",
		"apns-priority": "10",
		"apns-topic": "web.jim.tennis", // Your website's push identifier
	}, nil
}

// decodePrivateKey decodes a base64 URL-safe private key into an ECDSA private key
func decodePrivateKey(keyBytes []byte) (*ecdsa.PrivateKey, error) {
	// The key should be 32 bytes for P-256
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: %d", len(keyBytes))
	}

	// Create a new P-256 curve
	curve := elliptic.P256()

	// Convert the key bytes to a big integer
	d := new(big.Int).SetBytes(keyBytes)

	// Create the private key
	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
		},
		D: d,
	}

	// Compute the public key
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	return priv, nil
}

// signJWT signs the JWT using the VAPID private key
func signJWT(input, privateKey string) (string, error) {
	// Decode the private key
	keyBytes, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %v", err)
	}

	// Create a new ECDSA private key
	privKey, err := decodePrivateKey(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %v", err)
	}

	// Sign the input
	h := sha256.New()
	h.Write([]byte(input))
	signature, err := ecdsa.SignASN1(cryptorand.Reader, privKey, h.Sum(nil))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(signature), nil
}

// SendNotification sends a push notification to a subscription
func (s *Service) SendNotification(sub Subscription, message string) error {
	vapidPublic, vapidPrivate, err := s.GetVAPIDKeys()
	if err != nil {
		return err
	}

	// Create different payloads based on platform
	var payload []byte
	if sub.Platform == "safari" {
		// Create Safari-specific payload
		safariPayload := SafariNotificationPayload{
			Title: "Jim.Tennis",
			Body:  message,
			Icon:  "/static/icon-192.svg",
			Badge: "/static/icon-192.svg",
			Data: map[string]interface{}{
				"dateOfArrival": time.Now().Unix(),
				"url":          "https://jim.tennis",
			},
			Actions: []map[string]string{
				{
					"action": "open",
					"title":  "Open",
				},
				{
					"action": "close",
					"title":  "Close",
				},
			},
			Tag:      "default",
			Renotify: true,
		}
		payload, err = json.Marshal(safariPayload)
	} else {
		// Standard payload for other platforms
		payload, err = json.Marshal(map[string]string{
			"message":  message,
			"platform": sub.Platform,
		})
	}
	if err != nil {
		return err
	}

	// Log notification attempt
	log.Printf("Sending notification to %s platform subscription", sub.Platform)
	log.Printf("Endpoint: %s", sub.Endpoint)

	// Create webpush options
	options := &webpush.Options{
		VAPIDPublicKey:  vapidPublic,
		VAPIDPrivateKey: vapidPrivate,
		TTL:             30,
		Subscriber:      "mailto:admin@jim.tennis",
	}

	// For Safari, we need to create a custom HTTP client with the required headers
	var client *http.Client
	if sub.Platform == "safari" {
		headers, err := getSafariAuthHeaders(vapidPublic, vapidPrivate)
		if err != nil {
			log.Printf("Error generating Safari auth headers: %v", err)
			return err
		}
		log.Printf("Using Safari auth headers: %v", headers)

		// Create a custom transport that adds the headers
		transport := &http.Transport{}
		client = &http.Client{
			Transport: &customTransport{
				base:    transport,
				headers: headers,
			},
		}
	}

	// Send the notification
	var resp *http.Response
	if sub.Platform == "safari" && client != nil {
		// Use custom client for Safari
		resp, err = sendNotificationWithClient(payload, sub, options, client)
	} else {
		// Use default client for other platforms
		resp, err = webpush.SendNotification(
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
	}

	if err != nil {
		log.Printf("Error sending notification to %s: %v", sub.Platform, err)
		// Check for Safari-specific errors
		if sub.Platform == "safari" {
			if strings.Contains(err.Error(), "403") {
				log.Printf("Safari push notification rejected (403). This might be due to:")
				log.Printf("- Invalid VAPID keys for Safari")
				log.Printf("- Subscription expired")
				log.Printf("- Missing or invalid Safari push certificate")
				log.Printf("- Invalid JWT token")
				log.Printf("Full error details: %+v", err)
			}
		}
		return err
	}
	defer resp.Body.Close()

	// Read and log the response body for debugging
	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode >= 400 {
		log.Printf("Failed to send notification to %s, status: %d", sub.Platform, resp.StatusCode)
		// Handle Safari-specific status codes
		if sub.Platform == "safari" {
			switch resp.StatusCode {
			case http.StatusForbidden:
				log.Printf("Safari push notification forbidden (403). This might be due to:")
				log.Printf("- Invalid VAPID keys for Safari")
				log.Printf("- Subscription expired")
				log.Printf("- Missing or invalid Safari push certificate")
				log.Printf("- Invalid JWT token")
				log.Printf("Response body: %s", string(body))
			case http.StatusUnauthorized:
				log.Printf("Safari push notification unauthorized (401). This might be due to:")
				log.Printf("- Invalid authentication")
				log.Printf("- Expired VAPID keys")
				log.Printf("- Invalid JWT token")
				log.Printf("Response body: %s", string(body))
			}
		}
		// If the subscription is invalid (404) or expired (410), remove it
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			s.DeleteSubscription(sub.Endpoint)
		}
		return fmt.Errorf("failed to send notification, status: %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully sent notification to %s platform", sub.Platform)
	return nil
}

// customTransport adds custom headers to requests
type customTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add custom headers
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	return t.base.RoundTrip(req)
}

// sendNotificationWithClient sends a notification using a custom HTTP client
func sendNotificationWithClient(payload []byte, sub Subscription, options *webpush.Options, client *http.Client) (*http.Response, error) {
	// Create the request
	req, err := http.NewRequest("POST", sub.Endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}

	// Add required headers
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", fmt.Sprintf("%d", options.TTL))
	req.Header.Set("Urgency", "high")

	// Log the full request details
	log.Printf("Sending notification request:")
	log.Printf("  Method: %s", req.Method)
	log.Printf("  URL: %s", req.URL.String())
	log.Printf("  Headers:")
	for key, values := range req.Header {
		for _, value := range values {
			// Mask sensitive headers
			if strings.EqualFold(key, "Authorization") {
				log.Printf("    %s: %s", key, "Bearer [MASKED]")
			} else {
				log.Printf("    %s: %s", key, value)
			}
		}
	}
	log.Printf("  Payload: %s", string(payload))
	log.Printf("  Subscription details:")
	log.Printf("    Endpoint: %s", sub.Endpoint)
	log.Printf("    Platform: %s", sub.Platform)
	log.Printf("    P256dh: %s", sub.P256dh)
	log.Printf("    Auth: %s", sub.Auth)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return nil, err
	}

	// Log response details
	log.Printf("Response received:")
	log.Printf("  Status: %s", resp.Status)
	log.Printf("  Status Code: %d", resp.StatusCode)
	log.Printf("  Response Headers:")
	for key, values := range resp.Header {
		for _, value := range values {
			log.Printf("    %s: %s", key, value)
		}
	}

	// Read and log response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
	} else {
		log.Printf("Response Body: %s", string(body))
		// Create a new reader with the body for the response
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}

	return resp, nil
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