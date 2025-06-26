package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ServiceConfig holds the configuration for the webhook service
type ServiceConfig struct {
	Server   ServerConfig   `yaml:"server"`
	OpenFGA  OpenFGAConfig  `yaml:"openfga"`
	Auth0    Auth0Config    `yaml:"auth0"`
	Mappings MappingsConfig `yaml:"mappings"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int           `yaml:"port" env:"PORT" envDefault:"8080"`
	Host         string        `yaml:"host" env:"HOST" envDefault:"0.0.0.0"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env:"READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"WRITE_TIMEOUT" envDefault:"10s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"IDLE_TIMEOUT" envDefault:"120s"`
}

// OpenFGAConfig holds OpenFGA connection configuration
type OpenFGAConfig struct {
	APIUrl      string `yaml:"api_url" env:"OPENFGA_API_URL" envDefault:"http://localhost:8080"`
	StoreID     string `yaml:"store_id" env:"OPENFGA_STORE_ID"`
	ModelFile   string `yaml:"model_file" env:"OPENFGA_MODEL_FILE" envDefault:"configs/model.json"`
	AuthMethod  string `yaml:"auth_method" env:"OPENFGA_AUTH_METHOD" envDefault:"none"` // none, client_credentials, shared_secret
	ClientID    string `yaml:"client_id" env:"OPENFGA_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" env:"OPENFGA_CLIENT_SECRET"`
	SharedSecret string `yaml:"shared_secret" env:"OPENFGA_SHARED_SECRET"`
	Audience    string `yaml:"audience" env:"OPENFGA_AUDIENCE"`
	Issuer      string `yaml:"issuer" env:"OPENFGA_ISSUER"`
}

// Auth0Config holds Auth0 webhook configuration
type Auth0Config struct {
	WebhookSecret string `yaml:"webhook_secret" env:"AUTH0_WEBHOOK_SECRET"`
	VerifySignature bool  `yaml:"verify_signature" env:"AUTH0_VERIFY_SIGNATURE" envDefault:"true"`
}

// MappingsConfig holds the mapping configuration files
type MappingsConfig struct {
	UserMappings       string `yaml:"user_mappings" env:"USER_MAPPINGS_FILE" envDefault:"configs/user-mappings.yaml"`
	OrgMappings        string `yaml:"org_mappings" env:"ORG_MAPPINGS_FILE" envDefault:"configs/organization-mappings.yaml"`
	OrgMemberMappings  string `yaml:"org_member_mappings" env:"ORG_MEMBER_MAPPINGS_FILE" envDefault:"configs/organization-member-mappings.yaml"`
	OrgRoleMappings    string `yaml:"org_role_mappings" env:"ORG_ROLE_MAPPINGS_FILE" envDefault:"configs/organization-role-mappings.yaml"`
}

// LoadServiceConfig loads the service configuration from environment variables and config file
func LoadServiceConfig() (*ServiceConfig, error) {
	cfg := &ServiceConfig{}
	
	// Set defaults
	cfg.Server = ServerConfig{
		Port:         8080,
		Host:         "0.0.0.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	cfg.OpenFGA = OpenFGAConfig{
		APIUrl:     "http://localhost:8080",
		ModelFile:  "configs/model.json",
		AuthMethod: "none",
	}
	
	cfg.Auth0 = Auth0Config{
		VerifySignature: true,
	}
	
	cfg.Mappings = MappingsConfig{
		UserMappings:      "configs/user-mappings.yaml",
		OrgMappings:       "configs/organization-mappings.yaml",
		OrgMemberMappings: "configs/organization-member-mappings.yaml",
		OrgRoleMappings:   "configs/organization-role-mappings.yaml",
	}
	
	// Load from environment variables
	if err := loadFromEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to load from environment: %w", err)
	}
	
	return cfg, nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(cfg *ServiceConfig) error {
	// Server config
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}
	if host := os.Getenv("HOST"); host != "" {
		cfg.Server.Host = host
	}
	
	// OpenFGA config
	if apiUrl := os.Getenv("OPENFGA_API_URL"); apiUrl != "" {
		cfg.OpenFGA.APIUrl = apiUrl
	}
	if storeID := os.Getenv("OPENFGA_STORE_ID"); storeID != "" {
		cfg.OpenFGA.StoreID = storeID
	}
	if modelFile := os.Getenv("OPENFGA_MODEL_FILE"); modelFile != "" {
		cfg.OpenFGA.ModelFile = modelFile
	}
	if authMethod := os.Getenv("OPENFGA_AUTH_METHOD"); authMethod != "" {
		cfg.OpenFGA.AuthMethod = authMethod
	}
	if clientID := os.Getenv("OPENFGA_CLIENT_ID"); clientID != "" {
		cfg.OpenFGA.ClientID = clientID
	}
	if clientSecret := os.Getenv("OPENFGA_CLIENT_SECRET"); clientSecret != "" {
		cfg.OpenFGA.ClientSecret = clientSecret
	}
	if sharedSecret := os.Getenv("OPENFGA_SHARED_SECRET"); sharedSecret != "" {
		cfg.OpenFGA.SharedSecret = sharedSecret
	}
	if audience := os.Getenv("OPENFGA_AUDIENCE"); audience != "" {
		cfg.OpenFGA.Audience = audience
	}
	if issuer := os.Getenv("OPENFGA_ISSUER"); issuer != "" {
		cfg.OpenFGA.Issuer = issuer
	}
	
	// Auth0 config
	if webhookSecret := os.Getenv("AUTH0_WEBHOOK_SECRET"); webhookSecret != "" {
		cfg.Auth0.WebhookSecret = webhookSecret
	}
	if verifySignature := os.Getenv("AUTH0_VERIFY_SIGNATURE"); verifySignature != "" {
		cfg.Auth0.VerifySignature = verifySignature != "false"
	}
	
	// Mappings config
	if userMappings := os.Getenv("USER_MAPPINGS_FILE"); userMappings != "" {
		cfg.Mappings.UserMappings = userMappings
	}
	if orgMappings := os.Getenv("ORG_MAPPINGS_FILE"); orgMappings != "" {
		cfg.Mappings.OrgMappings = orgMappings
	}
	if orgMemberMappings := os.Getenv("ORG_MEMBER_MAPPINGS_FILE"); orgMemberMappings != "" {
		cfg.Mappings.OrgMemberMappings = orgMemberMappings
	}
	if orgRoleMappings := os.Getenv("ORG_ROLE_MAPPINGS_FILE"); orgRoleMappings != "" {
		cfg.Mappings.OrgRoleMappings = orgRoleMappings
	}
	
	return nil
}