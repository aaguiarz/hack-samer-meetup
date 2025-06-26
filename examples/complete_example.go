package main

import (
	"context"
	"fmt"
	"log"

	"mapping-engine/internal/config"
	"mapping-engine/internal/engine"
)

func main() {
	ctx := context.Background()

	// Load mapping configurations from YAML files
	configPaths := []string{
		"configs/user-mappings.yaml",
		"configs/organization-mappings.yaml", 
		"configs/organization-member-mappings.yaml",
		"configs/organization-role-mappings.yaml",
	}

	configs, err := config.LoadMappingConfigs(configPaths)
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	// Create multi-config processor
	processor := engine.NewMultiConfigProcessor(
		"http://localhost:8080", // OpenFGA API URL
		"store-id",              // Store ID
		"model-id",              // Model ID  
		configs,
	)

	// Example 1: User creation event
	fmt.Println("=== Processing User Creation Event ===")
	userCreateEvent := map[string]interface{}{
		"specversion": "1.0",
		"type":        "user.created",
		"source":      "urn:auth0:example.auth0app.com",
		"id":          "evt_user_created_001",
		"time":        "2025-06-26T12:00:00Z",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user_id":        "auth0|user123",
				"email":          "john.doe@example.com",
				"email_verified": true,
				"phone_verified": false,
				"blocked":        false,
				"app_metadata": map[string]interface{}{
					"manager": "auth0|manager456",
				},
			},
		},
	}

	err = processor.ProcessEvent(ctx, userCreateEvent)
	if err != nil {
		log.Printf("Error processing user create event: %v", err)
	} else {
		fmt.Println("User create event processed successfully")
	}

	// Example 2: User update event
	fmt.Println("\n=== Processing User Update Event ===")
	userUpdateEvent := map[string]interface{}{
		"type": "user.updated",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user_id":        "auth0|user123",
				"email_verified": true,
				"phone_verified": true, // Now verified
				"blocked":        true, // Now blocked
				// Manager removed from app_metadata
			},
		},
	}

	err = processor.ProcessEvent(ctx, userUpdateEvent)
	if err != nil {
		log.Printf("Error processing user update event: %v", err)
	} else {
		fmt.Println("User update event processed successfully")
	}

	// Example 3: Organization member added
	fmt.Println("\n=== Processing Organization Member Added Event ===")
	memberAddEvent := map[string]interface{}{
		"type": "organization.member.added",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user": map[string]interface{}{
					"user_id": "auth0|user123",
				},
				"organization": map[string]interface{}{
					"id": "org_company_abc",
				},
			},
		},
	}

	err = processor.ProcessEvent(ctx, memberAddEvent)
	if err != nil {
		log.Printf("Error processing member add event: %v", err)
	} else {
		fmt.Println("Member add event processed successfully")
	}

	// Example 4: Organization member role assigned
	fmt.Println("\n=== Processing Organization Member Role Assigned Event ===")
	roleAssignEvent := map[string]interface{}{
		"type": "organization.member.role.assigned",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user": map[string]interface{}{
					"user_id": "auth0|user123",
				},
				"role": map[string]interface{}{
					"id": "admin",
				},
				"organization": map[string]interface{}{
					"id": "org_company_abc",
				},
			},
		},
	}

	err = processor.ProcessEvent(ctx, roleAssignEvent)
	if err != nil {
		log.Printf("Error processing role assign event: %v", err)
	} else {
		fmt.Println("Role assign event processed successfully")
	}

	// Example 5: Organization creation with metadata
	fmt.Println("\n=== Processing Organization Creation Event ===")
	orgCreateEvent := map[string]interface{}{
		"type": "organization.created",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id": "org_company_abc",
				"metadata": map[string]interface{}{
					"external_org_id": "ext_org_999",
					"tier":            "premium",
				},
			},
		},
	}

	err = processor.ProcessEvent(ctx, orgCreateEvent)
	if err != nil {
		log.Printf("Error processing org create event: %v", err)
	} else {
		fmt.Println("Organization create event processed successfully")
	}

	fmt.Println("\n=== All events processed ===")
}

// demonstrateComplexScenario shows a complex scenario with multiple related events
func demonstrateComplexScenario() {
	fmt.Println("\n=== Complex Scenario Demonstration ===")
	
	// This would demonstrate:
	// 1. Creating a user with initial permissions
	// 2. Adding the user to an organization
	// 3. Assigning roles to the user in that organization
	// 4. Updating user properties
	// 5. Removing roles and memberships
	// 6. Finally deleting the user
	
	events := []string{
		"user.created",
		"organization.member.added", 
		"organization.member.role.assigned",
		"user.updated",
		"organization.member.role.deleted",
		"organization.member.deleted",
		"user.deleted",
	}
	
	fmt.Printf("Would process events in sequence: %v\n", events)
	fmt.Println("Each event would trigger appropriate OpenFGA tuple operations")
	fmt.Println("The final state would be clean with all tuples removed")
}
