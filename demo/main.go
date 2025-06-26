package main

import (
	"encoding/json"
	"fmt"
	"log"
	"text/template"
	"bytes"

	"github.com/antonmedv/expr"
	"mapping-engine/internal/types"
)

func main() {
	fmt.Println("=== Auth0 to OpenFGA Mapping Engine Demo ===\n")

	// Demo configuration
	config := &types.MappingConfig{
		Events: []types.EventMapping{
			{Type: "user.created", Action: "create"},
			{Type: "user.updated", Action: "update"},
			{Type: "user.deleted", Action: "delete"},
		},
		Mappings: []types.TupleMapping{
			{
				Condition: "data.object.email_verified == true",
				Tuple: types.TupleDefinition{
					User:     "user:{{ .data.object.user_id }}",
					Relation: "email_verified",
					Object:   "user:{{ .data.object.user_id }}",
				},
			},
			{
				Condition: "data.object.phone_verified == true",
				Tuple: types.TupleDefinition{
					User:     "user:{{ .data.object.user_id }}",
					Relation: "phone_verified",
					Object:   "user:{{ .data.object.user_id }}",
				},
			},
			{
				Condition: "data.object.blocked == true",
				Tuple: types.TupleDefinition{
					User:     "user:{{ .data.object.user_id }}",
					Relation: "blocked",
					Object:   "user:{{ .data.object.user_id }}",
				},
			},
			{
				Condition: "data.object.app_metadata != nil && data.object.app_metadata.manager != nil",
				Tuple: types.TupleDefinition{
					User:     "user:{{ .data.object.user_id }}",
					Relation: "manager",
					Object:   "user:{{ .data.object.app_metadata.manager }}",
				},
			},
		},
	}

	// Demo event
	eventJSON := `{
		"specversion": "1.0",
		"type": "user.updated",
		"source": "urn:auth0:example.auth0app.com",
		"id": "evt_1234567890abcdef",
		"time": "2025-06-26T12:34:56Z",
		"data": {
			"object": {
				"user_id": "auth0|507f1f77bcf86cd799439020",
				"email": "john.doe@gmail.com",
				"email_verified": true,
				"phone_verified": false,
				"blocked": true,
				"app_metadata": {
					"manager": "auth0|56780"
				}
			}
		},
		"a0tenant": "my-tenant",
		"a0stream": "est_1234567890abcdef"
	}`

	var event map[string]interface{}
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		log.Fatal(err)
	}

	fmt.Println("1. Input Auth0 Event:")
	prettyJSON, _ := json.MarshalIndent(event, "", "  ")
	fmt.Println(string(prettyJSON))

	fmt.Println("\n2. Mapping Configuration:")
	fmt.Printf("   - Event: %s → Action: %s\n", event["type"], findActionForEvent(config.Events, event["type"].(string)))
	fmt.Printf("   - %d mapping rules defined\n", len(config.Mappings))

	fmt.Println("\n3. Evaluating Mapping Rules:")

	// Demonstrate the mapping evaluation logic
	tuples, err := evaluateMappingsDemo(event, config.Mappings)
	if err != nil {
		log.Printf("Error evaluating mappings: %v", err)
		return
	}

	fmt.Printf("\n4. Generated OpenFGA Tuples (%d total):\n", len(tuples))
	for i, tuple := range tuples {
		fmt.Printf("   %d. User: %s, Relation: %s, Object: %s\n", 
			i+1, tuple.User, tuple.Relation, tuple.Object)
	}

	fmt.Println("\n5. What would happen with OpenFGA:")
	fmt.Println("   - Since this is an 'update' action, the engine would:")
	fmt.Println("     a) Read existing tuples from OpenFGA")
	fmt.Println("     b) Compare with new tuples")  
	fmt.Println("     c) Add missing tuples")
	fmt.Println("     d) Remove obsolete tuples")

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nTo run with a real OpenFGA server:")
	fmt.Println("1. Start OpenFGA: docker run -p 8080:8080 openfga/openfga:latest run")
	fmt.Println("2. Create store and model using tools/openfga-util.go")
	fmt.Println("3. Update the store and model IDs in the code")
	fmt.Println("4. Run the integration tests")
}

func findActionForEvent(events []types.EventMapping, eventType string) string {
	for _, event := range events {
		if event.Type == eventType {
			return event.Action
		}
	}
	return "unknown"
}

// evaluateMappingsDemo demonstrates the mapping evaluation logic
func evaluateMappingsDemo(event map[string]interface{}, mappings []types.TupleMapping) ([]types.ProcessedTuple, error) {
	var results []types.ProcessedTuple

	for i, mapping := range mappings {
		fmt.Printf("   Rule %d: %s\n", i+1, mapping.Condition)

		// Evaluate condition
		matches, err := evaluateCondition(mapping.Condition, event)
		if err != nil {
			fmt.Printf("      ❌ Error: %v\n", err)
			continue
		}

		if matches {
			fmt.Printf("      ✅ Condition matches\n")
			
			// Process templates
			processedTuple, err := processTemplates(mapping.Tuple, event)
			if err != nil {
				fmt.Printf("      ❌ Template error: %v\n", err)
				continue
			}

			fmt.Printf("      → Generated tuple: %s#%s@%s\n", 
				processedTuple.User, processedTuple.Relation, processedTuple.Object)
			results = append(results, processedTuple)
		} else {
			fmt.Printf("      ❌ Condition does not match\n")
		}
	}

	return results, nil
}

// Helper functions that replicate the engine logic for demo
func evaluateCondition(condition string, event map[string]interface{}) (bool, error) {
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

func processTemplates(tupleDefinition types.TupleDefinition, event map[string]interface{}) (types.ProcessedTuple, error) {
	user, err := processTemplate(tupleDefinition.User, event)
	if err != nil {
		return types.ProcessedTuple{}, fmt.Errorf("failed to process user template: %w", err)
	}

	relation, err := processTemplate(tupleDefinition.Relation, event)
	if err != nil {
		return types.ProcessedTuple{}, fmt.Errorf("failed to process relation template: %w", err)
	}

	object, err := processTemplate(tupleDefinition.Object, event)
	if err != nil {
		return types.ProcessedTuple{}, fmt.Errorf("failed to process object template: %w", err)
	}

	return types.ProcessedTuple{
		User:     user,
		Relation: relation,
		Object:   object,
	}, nil
}

func processTemplate(templateStr string, event map[string]interface{}) (string, error) {
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
