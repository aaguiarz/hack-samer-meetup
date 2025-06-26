package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mapping-engine/internal/config"
)

func TestWebhookService_Health(t *testing.T) {
	// Create test configuration
	cfg := &config.ServiceConfig{
		Server: config.ServerConfig{
			Port:         8080,
			Host:         "localhost",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		OpenFGA: config.OpenFGAConfig{
			APIUrl:     "http://localhost:8080",
			StoreID:    "test-store",
			ModelFile:  "../../configs/model.json",
			AuthMethod: "none",
		},
		Auth0: config.Auth0Config{
			VerifySignature: false, // Disable signature verification for tests
		},
		Mappings: config.MappingsConfig{
			UserMappings:      "../../configs/user-mappings.yaml",
			OrgMappings:       "../../configs/organization-mappings.yaml",
			OrgMemberMappings: "../../configs/organization-member-mappings.yaml",
			OrgRoleMappings:   "../../configs/organization-role-mappings.yaml",
		},
	}

	// Create service (without starting the server)
	svc := &WebhookService{
		cfg: cfg,
	}
	
	// Initialize router
	svc.router = mux.NewRouter()
	svc.setupRoutes()

	// Create test request
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	svc.router.ServeHTTP(rr, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "auth0-openfga-webhook", response["service"])
	assert.NotNil(t, response["timestamp"])
}

func TestWebhookService_Auth0Webhook_InvalidJSON(t *testing.T) {
	// Create test configuration
	cfg := &config.ServiceConfig{
		OpenFGA: config.OpenFGAConfig{
			APIUrl:     "http://localhost:8080",
			StoreID:    "test-store",
			ModelFile:  "../../configs/model.json",
			AuthMethod: "none",
		},
		Auth0: config.Auth0Config{
			VerifySignature: false,
		},
	}

	// Create service
	svc := &WebhookService{
		cfg: cfg,
	}
	
	// Initialize router
	svc.router = mux.NewRouter()
	svc.setupRoutes()

	// Create test request with invalid JSON
	req, err := http.NewRequest("POST", "/webhook/auth0", bytes.NewBufferString("invalid json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	svc.router.ServeHTTP(rr, req)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWebhookService_Auth0Webhook_MissingEventType(t *testing.T) {
	// Create test configuration
	cfg := &config.ServiceConfig{
		OpenFGA: config.OpenFGAConfig{
			APIUrl:     "http://localhost:8080",
			StoreID:    "test-store",
			ModelFile:  "../../configs/model.json",
			AuthMethod: "none",
		},
		Auth0: config.Auth0Config{
			VerifySignature: false,
		},
	}

	// Create service
	svc := &WebhookService{
		cfg: cfg,
	}
	
	// Initialize router
	svc.router = mux.NewRouter()
	svc.setupRoutes()

	// Create test event without type
	event := map[string]interface{}{
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user_id": "auth0|test-user",
			},
		},
	}

	eventJSON, _ := json.Marshal(event)
	req, err := http.NewRequest("POST", "/webhook/auth0", bytes.NewBuffer(eventJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	svc.router.ServeHTTP(rr, req)

	// Check the response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestWebhookService_Auth0Webhook_UnknownEventType(t *testing.T) {
	// Create test configuration
	cfg := &config.ServiceConfig{
		OpenFGA: config.OpenFGAConfig{
			APIUrl:     "http://localhost:8080",
			StoreID:    "test-store",
			ModelFile:  "../../configs/model.json",
			AuthMethod: "none",
		},
		Auth0: config.Auth0Config{
			VerifySignature: false,
		},
	}

	// Create service
	svc := &WebhookService{
		cfg: cfg,
	}
	
	// Initialize router
	svc.router = mux.NewRouter()
	svc.setupRoutes()

	// Create test event with unknown type
	event := map[string]interface{}{
		"type": "unknown.event.type",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user_id": "auth0|test-user",
			},
		},
	}

	eventJSON, _ := json.Marshal(event)
	req, err := http.NewRequest("POST", "/webhook/auth0", bytes.NewBuffer(eventJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	svc.router.ServeHTTP(rr, req)

	// Check the response - should succeed but do nothing for unknown events
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "processed", response["status"])
	assert.Equal(t, "unknown.event.type", response["event_type"])
}
