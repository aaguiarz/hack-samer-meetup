package engine

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/antonmedv/expr"
	"github.com/openfga/go-sdk/client"

	"mapping-engine/internal/types"
)

// MappingEngine handles the mapping of Auth0 events to OpenFGA tuples
type MappingEngine struct {
	fgaClient *client.OpenFgaClient
	storeID   string
	modelID   string
	isDryRun  bool // Added for mock mode
}

// MockMappingEngine is a dry-run version that doesn't make actual API calls
type MockMappingEngine struct {
	*MappingEngine
}

// NewMockMappingEngine creates a new mock mapping engine for dry-run mode
func NewMockMappingEngine(storeID, modelID string) *MappingEngine {
	return &MappingEngine{
		fgaClient: nil, // No actual client for dry-run
		storeID:   storeID,
		modelID:   modelID,
		isDryRun:  true,
	}
}

// NewMappingEngine creates a new mapping engine instance
func NewMappingEngine(apiURL, storeID, modelID string) *MappingEngine {
	configuration := &client.ClientConfiguration{
		ApiUrl:               apiURL,
		StoreId:              storeID,
		AuthorizationModelId: modelID,
	}

	fgaClient, _ := client.NewSdkClient(configuration)

	return &MappingEngine{
		fgaClient: fgaClient,
		storeID:   storeID,
		modelID:   modelID,
		isDryRun:  false,
	}
}

// NewMappingEngineWithClient creates a new mapping engine instance with a pre-configured client
func NewMappingEngineWithClient(fgaClient *client.OpenFgaClient, storeID, modelFile string) *MappingEngine {
	return &MappingEngine{
		fgaClient: fgaClient,
		storeID:   storeID,
		modelID:   modelFile,
		isDryRun:  false,
	}
}

// ProcessEventResult contains the result of processing an event
type ProcessEventResult struct {
	TuplesAdded   []types.ProcessedTuple
	TuplesDeleted []types.ProcessedTuple
	Action        string
	EventType     string
}

// ProcessEventWithDetails processes an event and returns detailed information about the operations
func (me *MappingEngine) ProcessEventWithDetails(ctx context.Context, event map[string]interface{}, config *types.MappingConfig) (*ProcessEventResult, error) {
	eventType, ok := event["type"].(string)
	if !ok {
		return nil, fmt.Errorf("event type not found or not a string")
	}

	// Find the action for this event type
	var action string
	for _, eventMapping := range config.Events {
		if eventMapping.Type == eventType {
			action = eventMapping.Action
			break
		}
	}

	if action == "" {
		return nil, fmt.Errorf("no action found for event type: %s", eventType)
	}

	result := &ProcessEventResult{
		Action:    action,
		EventType: eventType,
	}

	// Process mappings based on action
	switch action {
	case "create":
		tuples, err := me.evaluateMappings(event, config.Mappings)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate mappings: %w", err)
		}
		result.TuplesAdded = tuples
		if !me.isDryRun {
			err = me.processCreateEvent(ctx, event, config)
			if err != nil {
				return nil, err
			}
		}
	case "update":
		if me.isDryRun {
			// For dry-run, just evaluate mappings
			tuples, err := me.evaluateMappings(event, config.Mappings)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate mappings: %w", err)
			}
			result.TuplesAdded = tuples
		} else {
			// For real update, we need to calculate changes
			newTuples, err := me.evaluateMappings(event, config.Mappings)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate mappings: %w", err)
			}

			// Get existing tuples
			entityID, err := me.extractUserID(event)
			if err != nil {
				return nil, fmt.Errorf("failed to extract entity ID: %w", err)
			}

			existingTuples, err := me.readExistingTuples(ctx, entityID)
			if err != nil {
				return nil, fmt.Errorf("failed to read existing tuples: %w", err)
			}

			tuplesToAdd, tuplesToDelete := me.calculateTupleChanges(existingTuples, newTuples)
			result.TuplesAdded = tuplesToAdd
			result.TuplesDeleted = tuplesToDelete

			err = me.processUpdateEvent(ctx, event, config)
			if err != nil {
				return nil, err
			}
		}
	case "delete":
		tuples, err := me.evaluateMappings(event, config.Mappings)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate mappings: %w", err)
		}
		result.TuplesDeleted = tuples
		if !me.isDryRun {
			err = me.processDeleteEvent(ctx, event, config)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}

	return result, nil
}

// ProcessEvent processes an Auth0 event according to the mapping configuration
func (me *MappingEngine) ProcessEvent(ctx context.Context, event map[string]interface{}, config *types.MappingConfig) error {
	eventType, ok := event["type"].(string)
	if !ok {
		return fmt.Errorf("event type not found or not a string")
	}

	// Find the action for this event type
	var action string
	for _, eventMapping := range config.Events {
		if eventMapping.Type == eventType {
			action = eventMapping.Action
			break
		}
	}

	if action == "" {
		return fmt.Errorf("no action found for event type: %s", eventType)
	}

	// Process mappings based on action
	switch action {
	case "create":
		return me.processCreateEvent(ctx, event, config)
	case "update":
		return me.processUpdateEvent(ctx, event, config)
	case "delete":
		return me.processDeleteEvent(ctx, event, config)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// processCreateEvent handles create actions
func (me *MappingEngine) processCreateEvent(ctx context.Context, event map[string]interface{}, config *types.MappingConfig) error {
	tuples, err := me.evaluateMappings(event, config.Mappings)
	if err != nil {
		return fmt.Errorf("failed to evaluate mappings: %w", err)
	}

	if len(tuples) == 0 {
		return nil // No tuples to create
	}

	// Convert to OpenFGA tuples
	fgaTuples := make([]client.ClientTupleKey, len(tuples))
	for i, tuple := range tuples {
		fgaTuples[i] = client.ClientTupleKey{
			User:     tuple.User,
			Relation: tuple.Relation,
			Object:   tuple.Object,
		}
	}

	// Write tuples to OpenFGA
	body := client.ClientWriteRequest{
		Writes: fgaTuples,
	}

	options := client.ClientWriteOptions{
		StoreId: &me.storeID,
	}

	if me.isDryRun {
		// In dry-run mode, just log the action
		fmt.Printf("Dry-run: create tuples %v\n", fgaTuples)
		return nil
	}

	_, err = me.fgaClient.Write(ctx).Body(body).Options(options).Execute()
	if err != nil {
		return fmt.Errorf("failed to write tuples to OpenFGA: %w", err)
	}

	return nil
}

// processUpdateEvent handles update actions
func (me *MappingEngine) processUpdateEvent(ctx context.Context, event map[string]interface{}, config *types.MappingConfig) error {
	newTuples, err := me.evaluateMappings(event, config.Mappings)
	if err != nil {
		return fmt.Errorf("failed to evaluate mappings: %w", err)
	}

	// Get the user ID from the event to query existing tuples
	userID, err := me.extractUserID(event)
	if err != nil {
		return fmt.Errorf("failed to extract user ID: %w", err)
	}

	// Read existing tuples for this user
	existingTuples, err := me.readExistingTuples(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to read existing tuples: %w", err)
	}

	// Determine which tuples to add and which to delete
	tuplesToAdd, tuplesToDelete := me.calculateTupleChanges(existingTuples, newTuples)

	// Execute changes
	if len(tuplesToDelete) > 0 || len(tuplesToAdd) > 0 {
		body := client.ClientWriteRequest{}

		if len(tuplesToAdd) > 0 {
			fgaTuples := make([]client.ClientTupleKey, len(tuplesToAdd))
			for i, tuple := range tuplesToAdd {
				fgaTuples[i] = client.ClientTupleKey{
					User:     tuple.User,
					Relation: tuple.Relation,
					Object:   tuple.Object,
				}
			}
			body.Writes = fgaTuples
		}

		if len(tuplesToDelete) > 0 {
			fgaTuples := make([]client.ClientTupleKeyWithoutCondition, len(tuplesToDelete))
			for i, tuple := range tuplesToDelete {
				fgaTuples[i] = client.ClientTupleKeyWithoutCondition{
					User:     tuple.User,
					Relation: tuple.Relation,
					Object:   tuple.Object,
				}
			}
			body.Deletes = fgaTuples
		}

		options := client.ClientWriteOptions{
			StoreId: &me.storeID,
		}

		if me.isDryRun {
			// In dry-run mode, just log the action
			fmt.Printf("Dry-run: update tuples, add: %v, delete: %v\n", body.Writes, body.Deletes)
			return nil
		}

		_, err = me.fgaClient.Write(ctx).Body(body).Options(options).Execute()
		if err != nil {
			return fmt.Errorf("failed to update tuples in OpenFGA: %w", err)
		}
	}

	return nil
}

// processDeleteEvent handles delete actions
func (me *MappingEngine) processDeleteEvent(ctx context.Context, event map[string]interface{}, config *types.MappingConfig) error {
	// First, try to evaluate mappings to determine specific tuples to delete
	tuplesToDelete, err := me.evaluateMappings(event, config.Mappings)
	if err != nil {
		return fmt.Errorf("failed to evaluate mappings: %w", err)
	}

	// If we have specific tuples from mappings, delete those
	if len(tuplesToDelete) > 0 {
		// Convert to OpenFGA tuples for deletion
		fgaTuples := make([]client.ClientTupleKeyWithoutCondition, len(tuplesToDelete))
		for i, tuple := range tuplesToDelete {
			fgaTuples[i] = client.ClientTupleKeyWithoutCondition{
				User:     tuple.User,
				Relation: tuple.Relation,
				Object:   tuple.Object,
			}
		}

		body := client.ClientWriteRequest{
			Deletes: fgaTuples,
		}

		options := client.ClientWriteOptions{
			StoreId: &me.storeID,
		}

		if me.isDryRun {
			// In dry-run mode, just log the action
			fmt.Printf("Dry-run: delete tuples %v\n", fgaTuples)
			return nil
		}

		_, err = me.fgaClient.Write(ctx).Body(body).Options(options).Execute()
		if err != nil {
			return fmt.Errorf("failed to delete tuples from OpenFGA: %w", err)
		}

		return nil
	}

	// If no specific tuples were found from mappings, fall back to deleting all tuples for the entity
	// This handles cases like user.deleted or organization.deleted where we want to remove all related tuples
	userID, err := me.extractUserID(event)
	if err != nil {
		return fmt.Errorf("failed to extract user ID: %w", err)
	}

	// Read all existing tuples for this entity
	existingTuples, err := me.readExistingTuples(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to read existing tuples: %w", err)
	}

	if len(existingTuples) == 0 {
		return nil // No tuples to delete
	}

	// Delete all tuples for this entity
	fgaTuples := make([]client.ClientTupleKeyWithoutCondition, len(existingTuples))
	for i, tuple := range existingTuples {
		fgaTuples[i] = client.ClientTupleKeyWithoutCondition{
			User:     tuple.User,
			Relation: tuple.Relation,
			Object:   tuple.Object,
		}
	}

	body := client.ClientWriteRequest{
		Deletes: fgaTuples,
	}

	options := client.ClientWriteOptions{
		StoreId: &me.storeID,
	}

	if me.isDryRun {
		// In dry-run mode, just log the action
		fmt.Printf("Dry-run: delete all tuples for entity %s\n", userID)
		return nil
	}

	_, err = me.fgaClient.Write(ctx).Body(body).Options(options).Execute()
	if err != nil {
		return fmt.Errorf("failed to delete tuples from OpenFGA: %w", err)
	}

	return nil
}

// EvaluateMappings evaluates all mapping conditions and returns the resulting tuples
// This is a public method that exposes the internal evaluateMappings functionality
func (me *MappingEngine) EvaluateMappings(event map[string]interface{}, mappings []types.TupleMapping) ([]types.ProcessedTuple, error) {
	return me.evaluateMappings(event, mappings)
}

// evaluateMappings evaluates all mapping conditions and returns the resulting tuples
func (me *MappingEngine) evaluateMappings(event map[string]interface{}, mappings []types.TupleMapping) ([]types.ProcessedTuple, error) {
	var results []types.ProcessedTuple

	for _, mapping := range mappings {
		// Evaluate condition if present
		if mapping.Condition != "" {
			matches, err := me.evaluateCondition(mapping.Condition, event)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate condition '%s': %w", mapping.Condition, err)
			}
			if !matches {
				continue
			}
		}

		// Process templates
		processedTuple, err := me.processTemplates(mapping.Tuple, event)
		if err != nil {
			return nil, fmt.Errorf("failed to process templates: %w", err)
		}

		results = append(results, processedTuple)
	}

	return results, nil
}

// evaluateCondition evaluates a condition expression against the event data
func (me *MappingEngine) evaluateCondition(condition string, event map[string]interface{}) (bool, error) {
	program, err := expr.Compile(condition, expr.Env(event))
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, event)
	if err != nil {
		return false, err
	}

	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("condition did not evaluate to boolean")
	}

	return result, nil
}

// processTemplates processes Go templates in tuple definitions
func (me *MappingEngine) processTemplates(tupleDefinition types.TupleDefinition, event map[string]interface{}) (types.ProcessedTuple, error) {
	user, err := me.processTemplate(tupleDefinition.User, event)
	if err != nil {
		return types.ProcessedTuple{}, fmt.Errorf("failed to process user template: %w", err)
	}

	relation, err := me.processTemplate(tupleDefinition.Relation, event)
	if err != nil {
		return types.ProcessedTuple{}, fmt.Errorf("failed to process relation template: %w", err)
	}

	object, err := me.processTemplate(tupleDefinition.Object, event)
	if err != nil {
		return types.ProcessedTuple{}, fmt.Errorf("failed to process object template: %w", err)
	}

	return types.ProcessedTuple{
		User:     user,
		Relation: relation,
		Object:   object,
	}, nil
}

// processTemplate processes a single template string
func (me *MappingEngine) processTemplate(templateStr string, event map[string]interface{}) (string, error) {
	tmpl, err := template.New("tuple").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, event); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// extractUserID extracts the user ID from the event
func (me *MappingEngine) extractUserID(event map[string]interface{}) (string, error) {
	data, ok := event["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("data field not found or not an object")
	}

	object, ok := data["object"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("data.object field not found or not an object")
	}

	// Try user_id first (for user events)
	if userID, ok := object["user_id"].(string); ok {
		return userID, nil
	}

	// Try id field (for organization events)
	if id, ok := object["id"].(string); ok {
		return id, nil
	}

	// Try user.user_id (for organization member events)
	if userObj, ok := object["user"].(map[string]interface{}); ok {
		if userID, ok := userObj["user_id"].(string); ok {
			return userID, nil
		}
	}

	return "", fmt.Errorf("could not extract user/entity ID from event")
}

// readExistingTuples reads existing tuples for an entity from OpenFGA
func (me *MappingEngine) readExistingTuples(ctx context.Context, entityID string) ([]types.ProcessedTuple, error) {
	// Read all tuples without filtering by user first
	body := client.ClientReadRequest{}

	response, err := me.fgaClient.Read(ctx).Body(body).Execute()
	if err != nil {
		return nil, err
	}

	// Filter tuples that match the entity (could be user: or organization:)
	// For organizations, we need to find tuples where:
	// 1. User matches "organization:entityID" (e.g., organization has_tier tier)
	// 2. Object matches "organization:entityID" (e.g., external_org external_org organization)
	var tuples []types.ProcessedTuple
	userKey := fmt.Sprintf("user:%s", entityID)
	orgKey := fmt.Sprintf("organization:%s", entityID)

	for _, tuple := range response.Tuples {
		if tuple.Key.User == userKey || tuple.Key.User == orgKey || tuple.Key.Object == orgKey {
			tuples = append(tuples, types.ProcessedTuple{
				User:     tuple.Key.User,
				Relation: tuple.Key.Relation,
				Object:   tuple.Key.Object,
			})
		}
	}

	return tuples, nil
}

// calculateTupleChanges determines which tuples to add and which to delete
func (me *MappingEngine) calculateTupleChanges(existing, new []types.ProcessedTuple) ([]types.ProcessedTuple, []types.ProcessedTuple) {
	existingMap := make(map[string]types.ProcessedTuple)
	for _, tuple := range existing {
		key := fmt.Sprintf("%s#%s#%s", tuple.User, tuple.Relation, tuple.Object)
		existingMap[key] = tuple
	}

	newMap := make(map[string]types.ProcessedTuple)
	for _, tuple := range new {
		key := fmt.Sprintf("%s#%s#%s", tuple.User, tuple.Relation, tuple.Object)
		newMap[key] = tuple
	}

	var tuplesToAdd []types.ProcessedTuple
	var tuplesToDelete []types.ProcessedTuple

	// Find tuples to add (in new but not in existing)
	for key, tuple := range newMap {
		if _, exists := existingMap[key]; !exists {
			tuplesToAdd = append(tuplesToAdd, tuple)
		}
	}

	// Find tuples to delete (in existing but not in new)
	for key, tuple := range existingMap {
		if _, exists := newMap[key]; !exists {
			tuplesToDelete = append(tuplesToDelete, tuple)
		}
	}

	return tuplesToAdd, tuplesToDelete
}
