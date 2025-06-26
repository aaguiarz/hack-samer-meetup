package engine

import (
	"context"
	"fmt"

	"mapping-engine/internal/types"
)

// MultiConfigProcessor processes events against multiple mapping configurations
type MultiConfigProcessor struct {
	engine  *MappingEngine
	configs []*types.MappingConfig
}

// NewMultiConfigProcessor creates a new multi-config processor
func NewMultiConfigProcessor(apiURL, storeID, modelID string, configs []*types.MappingConfig) *MultiConfigProcessor {
	return &MultiConfigProcessor{
		engine:  NewMappingEngine(apiURL, storeID, modelID),
		configs: configs,
	}
}

// ProcessEvent processes an event against all applicable configurations
func (mcp *MultiConfigProcessor) ProcessEvent(ctx context.Context, event map[string]interface{}) error {
	eventType, ok := event["type"].(string)
	if !ok {
		return fmt.Errorf("event type not found or not a string")
	}

	// Find all configurations that handle this event type
	var applicableConfigs []*types.MappingConfig
	for _, config := range mcp.configs {
		for _, eventMapping := range config.Events {
			if eventMapping.Type == eventType {
				applicableConfigs = append(applicableConfigs, config)
				break
			}
		}
	}

	if len(applicableConfigs) == 0 {
		return fmt.Errorf("no configuration found for event type: %s", eventType)
	}

	// Process event with each applicable configuration
	for _, config := range applicableConfigs {
		if err := mcp.engine.ProcessEvent(ctx, event, config); err != nil {
			return fmt.Errorf("failed to process event with config: %w", err)
		}
	}

	return nil
}

// AddConfig adds a new mapping configuration
func (mcp *MultiConfigProcessor) AddConfig(config *types.MappingConfig) {
	mcp.configs = append(mcp.configs, config)
}

// GetConfigs returns all loaded configurations  
func (mcp *MultiConfigProcessor) GetConfigs() []*types.MappingConfig {
	return mcp.configs
}
