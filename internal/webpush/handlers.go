// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package webpush

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// SetupHandlers registers webpush HTTP handlers on the given mux.
//
// requireAdmin wraps the operator-only routes (broadcast to every subscriber,
// VAPID key rotation); in production it is RequireAuth + RequireRole("admin").
// Player self-service routes stay public — the fantasy token in the request is
// the credential.
func (s *Service) SetupHandlers(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	mux.HandleFunc("/api/vapid-public-key", s.handleGetVAPIDPublicKey)
	mux.HandleFunc("/api/push/subscribe", s.handleSubscribe)
	mux.HandleFunc("/api/push/unsubscribe", s.handleUnsubscribe)
	mux.HandleFunc("/api/push/test-player", s.handleTestPlayerPush)
	mux.Handle("/api/push/test", requireAdmin(http.HandlerFunc(s.handleTestPush)))
	mux.Handle("/api/vapid-reset", requireAdmin(http.HandlerFunc(s.handleResetVAPIDKeys)))
}

// handleGetVAPIDPublicKey returns the VAPID public key
func (s *Service) handleGetVAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try to get existing keys first
	publicKey, _, err := s.GetVAPIDKeys()
	if err != nil {
		if err.Error() == "no VAPID keys found" {
			log.Printf("No existing VAPID keys found, generating new ones...")
			// Only generate new keys if none exist
			publicKey, _, err = s.GenerateVAPIDKeys()
			if err != nil {
				log.Printf("Error generating VAPID keys: %v", err)
				http.Error(w, "Failed to get VAPID public key", http.StatusInternalServerError)
				return
			}
		} else {
			log.Printf("Error getting VAPID keys: %v", err)
			http.Error(w, "Failed to get VAPID public key", http.StatusInternalServerError)
			return
		}
	}

	log.Printf("Returning VAPID public key: %s", publicKey)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"publicKey": publicKey,
	})
}

// handleSubscribe handles subscription requests
func (s *Service) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var subReq SubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&subReq); err != nil {
		http.Error(w, "Invalid subscription data", http.StatusBadRequest)
		return
	}

	var playerToken *string
	if subReq.PlayerToken != "" {
		playerToken = &subReq.PlayerToken
	}

	subscription := &Subscription{
		Endpoint:    subReq.Endpoint,
		P256dh:      subReq.Keys.P256dh,
		Auth:        subReq.Keys.Auth,
		UserAgent:   r.UserAgent(),
		PlayerToken: playerToken,
		CreatedAt:   time.Now(),
	}

	if err := s.SaveSubscription(subscription); err != nil {
		log.Printf("Error saving subscription: %v", err)
		http.Error(w, "Failed to save subscription", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// handleUnsubscribe handles unsubscription requests
func (s *Service) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var subReq struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&subReq); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	if err := s.DeleteSubscription(subReq.Endpoint); err != nil {
		log.Printf("Error deleting subscription: %v", err)
		http.Error(w, "Failed to delete subscription", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// handleTestPush sends a test push notification to all subscribers
func (s *Service) handleTestPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqData struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		reqData.Message = "This is a test notification!"
		log.Printf("No message provided in request, using default: %s", reqData.Message)
	} else {
		log.Printf("Received test push request with message: %s", reqData.Message)
	}

	log.Printf("Starting test push notification broadcast...")
	startTime := time.Now()

	// Get subscription count before sending
	subs, err := s.GetAllSubscriptions()
	if err != nil {
		log.Printf("Error getting subscriptions: %v", err)
		http.Error(w, "Failed to get subscriptions", http.StatusInternalServerError)
		return
	}
	log.Printf("Found %d active subscriptions to notify", len(subs))

	// Send notifications in a goroutine
	go func() {
		if err := s.SendToAll(reqData.Message); err != nil {
			log.Printf("Error during test push broadcast: %v", err)
		}
		duration := time.Since(startTime)
		log.Printf("Test push broadcast completed in %v", duration)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "success",
		"message":           "Notifications are being sent",
		"subscriptionCount": len(subs),
		"startTime":         startTime.Format(time.RFC3339),
	})
}

// handleResetVAPIDKeys resets the VAPID keys
func (s *Service) handleResetVAPIDKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publicKey, privateKey, err := s.ResetVAPIDKeys()
	if err != nil {
		log.Printf("Error resetting VAPID keys: %v", err)
		http.Error(w, "Failed to reset VAPID keys", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"publicKey":  publicKey,
		"privateKey": privateKey[:10] + "..." + privateKey[len(privateKey)-10:], // Only show part of private key
	})
}

// handleTestPlayerPush sends a test notification to a specific player's devices
func (s *Service) handleTestPlayerPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PlayerToken string `json:"playerToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlayerToken == "" {
		http.Error(w, "Missing playerToken", http.StatusBadRequest)
		return
	}

	payload := map[string]interface{}{
		"title": "Test Notification",
		"body":  "Your notifications are working! You'll receive alerts when selected for matches.",
		"data": map[string]string{
			"type": "test",
			"url":  "/my-availability/" + req.PlayerToken,
		},
	}

	sent, err := s.SendToPlayer(req.PlayerToken, payload)
	if err != nil {
		log.Printf("Error sending test to player %s: %v", req.PlayerToken, err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"sent":   sent,
	})
}
