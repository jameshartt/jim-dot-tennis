package webpush

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// SetupHandlers registers the webpush handlers
func (s *Service) SetupHandlers() {
	http.HandleFunc("/api/vapid-public-key", s.handleGetVAPIDPublicKey)
	http.HandleFunc("/api/push/subscribe", s.handleSubscribe)
	http.HandleFunc("/api/push/unsubscribe", s.handleUnsubscribe)
	http.HandleFunc("/api/push/test", s.handleTestPush)
	http.HandleFunc("/api/vapid-reset", s.handleResetVAPIDKeys)
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

	subscription := &Subscription{
		Endpoint:  subReq.Endpoint,
		P256dh:    subReq.Keys.P256dh,
		Auth:      subReq.Keys.Auth,
		UserAgent: r.UserAgent(),
		CreatedAt: time.Now(),
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
	}

	go func() {
		if err := s.SendToAll(reqData.Message); err != nil {
			log.Printf("Error sending notifications: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"message": "Notifications are being sent",
	})
}

// handleResetVAPIDKeys resets the VAPID keys
func (s *Service) handleResetVAPIDKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only allow from localhost or with admin authentication
	if !isLocalhost(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
		"status": "success",
		"publicKey": publicKey,
		"privateKey": privateKey[:10] + "..." + privateKey[len(privateKey)-10:], // Only show part of private key
	})
}

// isLocalhost checks if the request is from localhost
func isLocalhost(r *http.Request) bool {
	host := r.Host
	return host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "localhost:") || strings.HasPrefix(host, "127.0.0.1:")
} 