package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"

	"mapping-engine/internal/config"
	"mapping-engine/internal/engine"
	"mapping-engine/internal/types"
)

// WebhookService handles Auth0 webhook events and processes them through the mapping engine
type WebhookService struct {
	cfg           *config.ServiceConfig
	server        *http.Server
	router        *mux.Router
	mappingEngine *engine.MappingEngine
	fgaClient     *client.OpenFgaClient
	
	// Loaded mapping configurations
	userConfig     *types.MappingConfig
	orgConfig      *types.MappingConfig
	orgMemberConfig *types.MappingConfig
	orgRoleConfig   *types.MappingConfig
}

// NewWebhookService creates a new webhook service instance
func NewWebhookService(cfg *config.ServiceConfig) (*WebhookService, error) {
	svc := &WebhookService{
		cfg:    cfg,
		router: mux.NewRouter(),
	}

	// Initialize OpenFGA client
	if err := svc.initOpenFGAClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenFGA client: %w", err)
	}

	// Initialize mapping engine
	svc.mappingEngine = engine.NewMappingEngineWithClient(svc.fgaClient, cfg.OpenFGA.StoreID, cfg.OpenFGA.ModelFile)

	// Load mapping configurations
	if err := svc.loadMappingConfigs(); err != nil {
		return nil, fmt.Errorf("failed to load mapping configurations: %w", err)
	}

	// Setup routes
	svc.setupRoutes()

	// Create HTTP server
	svc.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      svc.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return svc, nil
}

// initOpenFGAClient initializes the OpenFGA client with the configured authentication
func (s *WebhookService) initOpenFGAClient() error {
	configuration := &client.ClientConfiguration{
		ApiUrl: s.cfg.OpenFGA.APIUrl,
	}

	// Configure authentication based on the auth method
	switch s.cfg.OpenFGA.AuthMethod {
	case "client_credentials":
		if s.cfg.OpenFGA.ClientID == "" || s.cfg.OpenFGA.ClientSecret == "" {
			return fmt.Errorf("client_id and client_secret are required for client_credentials auth")
		}
		configuration.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodClientCredentials,
			Config: &credentials.Config{
				ClientCredentialsClientId:     s.cfg.OpenFGA.ClientID,
				ClientCredentialsClientSecret: s.cfg.OpenFGA.ClientSecret,
				ClientCredentialsScopes:       "read write",
			},
		}
		if s.cfg.OpenFGA.Issuer != "" {
			configuration.Credentials.Config.ClientCredentialsApiTokenIssuer = s.cfg.OpenFGA.Issuer
		}

	case "shared_secret":
		if s.cfg.OpenFGA.SharedSecret == "" {
			return fmt.Errorf("shared_secret is required for shared_secret auth")
		}
		configuration.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: s.cfg.OpenFGA.SharedSecret,
			},
		}

	case "none":
		// No authentication
		break

	default:
		return fmt.Errorf("unsupported auth method: %s", s.cfg.OpenFGA.AuthMethod)
	}

	// Create the OpenFGA client
	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return fmt.Errorf("failed to create OpenFGA client: %w", err)
	}

	s.fgaClient = fgaClient
	return nil
}

// loadMappingConfigs loads all mapping configuration files
func (s *WebhookService) loadMappingConfigs() error {
	var err error

	// Load user mappings
	s.userConfig, err = config.LoadMappingConfig(s.cfg.Mappings.UserMappings)
	if err != nil {
		return fmt.Errorf("failed to load user mappings: %w", err)
	}

	// Load organization mappings
	s.orgConfig, err = config.LoadMappingConfig(s.cfg.Mappings.OrgMappings)
	if err != nil {
		return fmt.Errorf("failed to load organization mappings: %w", err)
	}

	// Load organization member mappings
	s.orgMemberConfig, err = config.LoadMappingConfig(s.cfg.Mappings.OrgMemberMappings)
	if err != nil {
		return fmt.Errorf("failed to load organization member mappings: %w", err)
	}

	// Load organization role mappings
	s.orgRoleConfig, err = config.LoadMappingConfig(s.cfg.Mappings.OrgRoleMappings)
	if err != nil {
		return fmt.Errorf("failed to load organization role mappings: %w", err)
	}

	return nil
}

// setupRoutes configures the HTTP routes
func (s *WebhookService) setupRoutes() {
	// Health check endpoint
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Auth0 webhook endpoint
	s.router.HandleFunc("/webhook/auth0", s.handleAuth0Webhook).Methods("POST")

	// Add middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.recoveryMiddleware)
}

// Start starts the webhook service
func (s *WebhookService) Start() error {
	log.Printf("Starting webhook service on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the webhook service
func (s *WebhookService) Shutdown(ctx context.Context) error {
	log.Println("Shutting down webhook service...")
	return s.server.Shutdown(ctx)
}

// handleHealth handles health check requests
func (s *WebhookService) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "auth0-openfga-webhook",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleAuth0Webhook handles Auth0 webhook events
func (s *WebhookService) handleAuth0Webhook(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify webhook signature if configured
	if s.cfg.Auth0.VerifySignature && s.cfg.Auth0.WebhookSecret != "" {
		if !s.verifyWebhookSignature(r, body) {
			log.Println("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse the webhook event
	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Failed to parse webhook event: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process the event
	if err := s.processEvent(r.Context(), event); err != nil {
		log.Printf("Failed to process webhook event: %v", err)
		http.Error(w, "Failed to process event", http.StatusInternalServerError)
		return
	}

	// Return success response
	response := map[string]interface{}{
		"status":     "processed",
		"timestamp":  time.Now().UTC(),
		"event_type": event["type"],
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// verifyWebhookSignature verifies the Auth0 webhook signature
func (s *WebhookService) verifyWebhookSignature(r *http.Request, body []byte) bool {
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		return false
	}

	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(s.cfg.Auth0.WebhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// processEvent processes a webhook event using the appropriate mapping configuration
func (s *WebhookService) processEvent(ctx context.Context, event map[string]interface{}) error {
	eventType, ok := event["type"].(string)
	if !ok {
		return fmt.Errorf("event type not found or not a string")
	}

	log.Printf("Processing event: %s", eventType)

	// Determine which mapping configuration to use based on event type
	var mappingConfig *types.MappingConfig
	switch {
	case strings.HasPrefix(eventType, "user."):
		mappingConfig = s.userConfig
	case strings.HasPrefix(eventType, "organization.") && !strings.Contains(eventType, "member"):
		mappingConfig = s.orgConfig
	case strings.Contains(eventType, "organization.member.role"):
		mappingConfig = s.orgRoleConfig
	case strings.Contains(eventType, "organization.member"):
		mappingConfig = s.orgMemberConfig
	default:
		log.Printf("No mapping configuration found for event type: %s", eventType)
		return nil // Not an error, just ignore unknown event types
	}

	// Process the event through the mapping engine
	if err := s.mappingEngine.ProcessEvent(ctx, event, mappingConfig); err != nil {
		return fmt.Errorf("mapping engine failed to process event: %w", err)
	}

	return nil
}

// loggingMiddleware logs all HTTP requests
func (s *WebhookService) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

// recoveryMiddleware recovers from panics and returns a 500 error
func (s *WebhookService) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
