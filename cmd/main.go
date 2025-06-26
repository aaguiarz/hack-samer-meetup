package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"mapping-engine/internal/engine"
	"mapping-engine/internal/types"
)

func main() {
	// Example usage
	mappingConfig := &types.MappingConfig{
		Events: []types.EventMapping{
			{
				Type:   "user.created",
				Action: "create",
			},
			{
				Type:   "user.updated",
				Action: "update",
			},
			{
				Type:   "user.deleted",
				Action: "delete",
			},
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
		},
	}

	// Sample Auth0 event
	eventJSON := `{
		"specversion": "1.0",
		"type": "user.updated",
		"source": "urn:auth0:example.auth0app.com", 
		"id": "evt_1234567890abcdef",
		"time": "2025-02-01T12:34:56Z",
		"data": {
			"object": {
				"user_id": "auth0|507f1f77bcf86cd799439020",
				"email": "john.doe@gmail.com",
				"email_verified": true,
				"phone_verified": false,
				"blocked": true
			}
		}
	}`

	var event map[string]interface{}
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		log.Fatal(err)
	}

	mappingEngine := engine.NewMappingEngine("http://localhost:8080", "store-id", "model-id")

	ctx := context.Background()
	err := mappingEngine.ProcessEvent(ctx, event, mappingConfig)
	if err != nil {
		log.Printf("Error processing event: %v", err)
	} else {
		fmt.Println("Event processed successfully")
	}
}
