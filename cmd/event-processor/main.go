package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"mapping-engine/internal/config"
	"mapping-engine/internal/engine"
	"mapping-engine/internal/types"
)

type CLIConfig struct {
	EventsFile        string
	OpenFGAURL        string
	StoreID           string
	ModelID           string
	ModelFile         string
	AuthMethod        string
	ClientID          string
	ClientSecret      string
	SharedSecret      string
	Audience          string
	Issuer            string
	Verbose           bool
	DryRun            bool
	UserMappings      string
	OrgMappings       string
	OrgMemberMappings string
	OrgRoleMappings   string
}

type EventProcessor struct {
	engine         *engine.MappingEngine
	userConfig     *types.MappingConfig
	orgConfig      *types.MappingConfig
	orgMemberConfig *types.MappingConfig
	orgRoleConfig   *types.MappingConfig
	verbose        bool
	dryRun         bool
}

type ProcessingResult struct {
	EventType     string                 `json:"event_type"`
	Success       bool                   `json:"success"`
	Error         string                 `json:"error,omitempty"`
	TuplesAdded   []types.ProcessedTuple `json:"tuples_added,omitempty"`
	TuplesDeleted []types.ProcessedTuple `json:"tuples_deleted,omitempty"`
	Duration      time.Duration          `json:"duration"`
}

func main() {
	cfg := parseFlags()
	
	if cfg.EventsFile == "" {
		log.Fatal("Events file is required. Use -events flag.")
	}
	
	// Load events from JSON file
	events, err := loadEventsFromFile(cfg.EventsFile)
	if err != nil {
		log.Fatalf("Failed to load events from file: %v", err)
	}
	
	fmt.Printf("🚀 Auth0 to OpenFGA Event Processor\n")
	fmt.Printf("====================================\n")
	fmt.Printf("📁 Events file: %s\n", cfg.EventsFile)
	fmt.Printf("🎯 OpenFGA URL: %s\n", cfg.OpenFGAURL)
	fmt.Printf("🏪 Store ID: %s\n", cfg.StoreID)
	fmt.Printf("🔧 Model ID: %s\n", cfg.ModelID)
	fmt.Printf("📊 Total events: %d\n", len(events))
	if cfg.DryRun {
		fmt.Printf("🔍 DRY RUN MODE - No changes will be made\n")
	}
	fmt.Printf("\n")
	
	// Create event processor
	processor, err := NewEventProcessor(cfg)
	if err != nil {
		log.Fatalf("Failed to create event processor: %v", err)
	}
	
	// Process all events
	results := processor.ProcessEvents(context.Background(), events)
	
	// Print summary
	printSummary(results)
}

func parseFlags() *CLIConfig {
	cfg := &CLIConfig{}
	
	flag.StringVar(&cfg.EventsFile, "events", "", "Path to JSON file containing Auth0 events")
	flag.StringVar(&cfg.OpenFGAURL, "openfga-url", getEnvOrDefault("OPENFGA_API_URL", "http://localhost:8080"), "OpenFGA API URL")
	flag.StringVar(&cfg.StoreID, "store-id", getEnvOrDefault("OPENFGA_STORE_ID", ""), "OpenFGA Store ID")
	flag.StringVar(&cfg.ModelID, "model-id", getEnvOrDefault("OPENFGA_MODEL_ID", ""), "OpenFGA Authorization Model ID")
	flag.StringVar(&cfg.ModelFile, "model-file", getEnvOrDefault("OPENFGA_MODEL_FILE", "configs/model.json"), "OpenFGA model file")
	flag.StringVar(&cfg.AuthMethod, "auth-method", getEnvOrDefault("OPENFGA_AUTH_METHOD", "none"), "Authentication method (none, client_credentials, shared_secret)")
	flag.StringVar(&cfg.ClientID, "client-id", getEnvOrDefault("OPENFGA_CLIENT_ID", ""), "OAuth2 Client ID")
	flag.StringVar(&cfg.ClientSecret, "client-secret", getEnvOrDefault("OPENFGA_CLIENT_SECRET", ""), "OAuth2 Client Secret")
	flag.StringVar(&cfg.SharedSecret, "shared-secret", getEnvOrDefault("OPENFGA_SHARED_SECRET", ""), "Shared secret for API token auth")
	flag.StringVar(&cfg.Audience, "audience", getEnvOrDefault("OPENFGA_AUDIENCE", ""), "OAuth2 audience")
	flag.StringVar(&cfg.Issuer, "issuer", getEnvOrDefault("OPENFGA_ISSUER", ""), "OAuth2 token issuer")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Show what would be done without making changes")
	flag.StringVar(&cfg.UserMappings, "user-mappings", "configs/user-mappings.yaml", "User mappings file")
	flag.StringVar(&cfg.OrgMappings, "org-mappings", "configs/organization-mappings.yaml", "Organization mappings file")
	flag.StringVar(&cfg.OrgMemberMappings, "org-member-mappings", "configs/organization-member-mappings.yaml", "Organization member mappings file")
	flag.StringVar(&cfg.OrgRoleMappings, "org-role-mappings", "configs/organization-role-mappings.yaml", "Organization role mappings file")
	
	flag.Parse()
	
	return cfg
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func loadEventsFromFile(filename string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	var events []map[string]interface{}
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	return events, nil
}

func NewEventProcessor(cfg *CLIConfig) (*EventProcessor, error) {
	// Create mapping engine based on configuration
	var mappingEngine *engine.MappingEngine
	var err error
	
	if cfg.DryRun {
		// For dry run, we'll create a mock engine that doesn't actually write to OpenFGA
		mappingEngine = engine.NewMockMappingEngine(cfg.StoreID, cfg.ModelID)
	} else {
		// Create real mapping engine
		mappingEngine = engine.NewMappingEngine(cfg.OpenFGAURL, cfg.StoreID, cfg.ModelID)
		
		// Configure authentication if needed
		if cfg.AuthMethod != "none" {
			err = configureMappingEngineAuth(mappingEngine, cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to configure authentication: %w", err)
			}
		}
	}
	
	// Load mapping configurations
	userConfig, err := config.LoadMappingConfig(cfg.UserMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to load user mappings: %w", err)
	}
	
	orgConfig, err := config.LoadMappingConfig(cfg.OrgMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization mappings: %w", err)
	}
	
	orgMemberConfig, err := config.LoadMappingConfig(cfg.OrgMemberMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization member mappings: %w", err)
	}
	
	orgRoleConfig, err := config.LoadMappingConfig(cfg.OrgRoleMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization role mappings: %w", err)
	}
	
	return &EventProcessor{
		engine:          mappingEngine,
		userConfig:      userConfig,
		orgConfig:       orgConfig,
		orgMemberConfig: orgMemberConfig,
		orgRoleConfig:   orgRoleConfig,
		verbose:         cfg.Verbose,
		dryRun:          cfg.DryRun,
	}, nil
}

func configureMappingEngineAuth(engine *engine.MappingEngine, cfg *CLIConfig) error {
	// This would configure authentication on the engine
	// For now, we'll assume the engine handles this internally
	return nil
}

func (ep *EventProcessor) ProcessEvents(ctx context.Context, events []map[string]interface{}) []ProcessingResult {
	results := make([]ProcessingResult, 0, len(events))
	
	for i, event := range events {
		fmt.Printf("[%d/%d] ", i+1, len(events))
		result := ep.processEvent(ctx, event)
		results = append(results, result)
		
		if ep.verbose || !result.Success {
			ep.printEventResult(result)
		} else {
			ep.printEventSummary(result)
		}
		
		// Small delay to make output readable
		time.Sleep(100 * time.Millisecond)
	}
	
	return results
}

func (ep *EventProcessor) processEvent(ctx context.Context, event map[string]interface{}) ProcessingResult {
	start := time.Now()
	
	eventType, ok := event["type"].(string)
	if !ok {
		return ProcessingResult{
			EventType: "unknown",
			Success:   false,
			Error:     "event type not found or not a string",
			Duration:  time.Since(start),
		}
	}
	
	result := ProcessingResult{
		EventType: eventType,
		Duration:  time.Since(start),
	}
	
	// Select appropriate mapping configuration
	var mappingConfig *types.MappingConfig
	switch {
	case strings.HasPrefix(eventType, "user."):
		mappingConfig = ep.userConfig
	case strings.HasPrefix(eventType, "organization.") && !strings.Contains(eventType, "member"):
		mappingConfig = ep.orgConfig
	case strings.Contains(eventType, "organization.member.role"):
		mappingConfig = ep.orgRoleConfig
	case strings.Contains(eventType, "organization.member"):
		mappingConfig = ep.orgMemberConfig
	default:
		result.Success = false
		result.Error = fmt.Sprintf("no mapping configuration found for event type: %s", eventType)
		return result
	}
	
	// Process the event using the engine
	processResult, err := ep.engine.ProcessEventWithDetails(ctx, event, mappingConfig)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
		result.TuplesAdded = processResult.TuplesAdded
		result.TuplesDeleted = processResult.TuplesDeleted
	}
	
	result.Duration = time.Since(start)
	return result
}

func (ep *EventProcessor) printEventSummary(result ProcessingResult) {
	status := "✅"
	if !result.Success {
		status = "❌"
	}
	
	fmt.Printf("%s %s (%v)\n", status, result.EventType, result.Duration)
	
	if !result.Success && result.Error != "" {
		fmt.Printf("   Error: %s\n", result.Error)
	}
}

func (ep *EventProcessor) printEventResult(result ProcessingResult) {
	status := "✅ SUCCESS"
	if !result.Success {
		status = "❌ FAILED"
	}
	
	fmt.Printf("%s - %s (%v)\n", status, result.EventType, result.Duration)
	
	if result.Error != "" {
		fmt.Printf("   Error: %s\n", result.Error)
	}
	
	if len(result.TuplesAdded) > 0 {
		fmt.Printf("   📝 Tuples Added:\n")
		for _, tuple := range result.TuplesAdded {
			fmt.Printf("      + %s %s %s\n", tuple.User, tuple.Relation, tuple.Object)
		}
	}
	
	if len(result.TuplesDeleted) > 0 {
		fmt.Printf("   🗑️ Tuples Deleted:\n")
		for _, tuple := range result.TuplesDeleted {
			fmt.Printf("      - %s %s %s\n", tuple.User, tuple.Relation, tuple.Object)
		}
	}
	
	fmt.Println()
}

func printSummary(results []ProcessingResult) {
	fmt.Printf("\n📊 Processing Summary\n")
	fmt.Printf("====================\n")
	
	successful := 0
	failed := 0
	totalTuplesAdded := 0
	totalTuplesDeleted := 0
	totalDuration := time.Duration(0)
	
	eventTypeCounts := make(map[string]int)
	
	for _, result := range results {
		if result.Success {
			successful++
		} else {
			failed++
		}
		
		totalTuplesAdded += len(result.TuplesAdded)
		totalTuplesDeleted += len(result.TuplesDeleted)
		totalDuration += result.Duration
		
		eventTypeCounts[result.EventType]++
	}
	
	fmt.Printf("�� Total Events: %d\n", len(results))
	fmt.Printf("✅ Successful: %d\n", successful)
	fmt.Printf("❌ Failed: %d\n", failed)
	fmt.Printf("📝 Total Tuples Added: %d\n", totalTuplesAdded)
	fmt.Printf("🗑️ Total Tuples Deleted: %d\n", totalTuplesDeleted)
	fmt.Printf("⏱️ Total Duration: %v\n", totalDuration)
	fmt.Printf("📊 Average Duration: %v\n", totalDuration/time.Duration(len(results)))
	
	fmt.Printf("\n📋 Event Types Processed:\n")
	for eventType, count := range eventTypeCounts {
		fmt.Printf("   %s: %d events\n", eventType, count)
	}
	
	if failed > 0 {
		fmt.Printf("\n❌ Failed Events:\n")
		for _, result := range results {
			if !result.Success {
				fmt.Printf("   %s: %s\n", result.EventType, result.Error)
			}
		}
	}
	
	fmt.Printf("\n🎉 Processing completed!\n")
}
