package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	fgaSdk "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"mapping-engine/internal/config"
	"mapping-engine/internal/types"
)

const (
	openfgaImage = "openfga/openfga:latest"
)

func init() {
	// Disable ryuk container to avoid Docker authentication issues
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
}

// TestContainer wraps the OpenFGA container for testing
type TestContainer struct {
	container testcontainers.Container
	host      string
	port      string
	apiURL    string
}

// setupOpenFGAContainer starts an OpenFGA container for testing
func setupOpenFGAContainer(ctx context.Context) (*TestContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        openfgaImage,
		ExposedPorts: []string{"8080/tcp"},
		Cmd:          []string{"run", "--playground-enabled=false"},
		WaitingFor: wait.ForHTTP("/healthz").
			WithPort("8080").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	return &TestContainer{
		container: container,
		host:      host,
		port:      port.Port(),
		apiURL:    apiURL,
	}, nil
}

// Close terminates the container
func (tc *TestContainer) Close() error {
	return tc.container.Terminate(context.Background())
}

// createTestStore creates a store for testing and returns the store ID
func (tc *TestContainer) createTestStore(ctx context.Context, storeName string) (string, error) {
	configuration := &client.ClientConfiguration{
		ApiUrl: tc.apiURL,
	}

	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return "", err
	}

	body := client.ClientCreateStoreRequest{
		Name: storeName,
	}

	response, err := fgaClient.CreateStore(ctx).Body(body).Execute()
	if err != nil {
		return "", err
	}

	return response.Id, nil
}

// createTestModel creates an authorization model and returns the model ID
func (tc *TestContainer) createTestModel(ctx context.Context, storeID string) (string, error) {
	configuration := &client.ClientConfiguration{
		ApiUrl:  tc.apiURL,
		StoreId: storeID,
	}

	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return "", err
	}

	// Load model from JSON file
	modelData, err := os.ReadFile("../../configs/model.json")
	if err != nil {
		return "", err
	}

	var authModel struct {
		SchemaVersion   string                   `json:"schema_version"`
		TypeDefinitions []map[string]interface{} `json:"type_definitions"`
	}

	err = json.Unmarshal(modelData, &authModel)
	if err != nil {
		return "", err
	}

	// Convert to fgaSdk.TypeDefinition
	var typeDefinitions []fgaSdk.TypeDefinition
	for _, typeDef := range authModel.TypeDefinitions {
		td := fgaSdk.TypeDefinition{
			Type: typeDef["type"].(string),
		}

		// Handle relations if they exist
		if relationsInterface, hasRelations := typeDef["relations"]; hasRelations {
			if relationsMap, ok := relationsInterface.(map[string]interface{}); ok {
				relations := make(map[string]fgaSdk.Userset)

				for relationName, relationDef := range relationsMap {
					if relationDefMap, ok := relationDef.(map[string]interface{}); ok {
						userset := fgaSdk.Userset{}

						// Handle "this" relation
						if _, hasThis := relationDefMap["this"]; hasThis {
							emptyMap := make(map[string]interface{})
							userset.This = &emptyMap
						}

						relations[relationName] = userset
					}
				}

				if len(relations) > 0 {
					td.Relations = &relations
				}
			}
		}

		// Handle metadata if it exists
		if metadataInterface, hasMetadata := typeDef["metadata"]; hasMetadata {
			if metadataMap, ok := metadataInterface.(map[string]interface{}); ok {
				// Convert metadata to fgaSdk.Metadata
				metadataBytes, err := json.Marshal(metadataMap)
				if err != nil {
					return "", err
				}

				var metadata fgaSdk.Metadata
				err = json.Unmarshal(metadataBytes, &metadata)
				if err != nil {
					return "", err
				}

				td.Metadata = &metadata
			}
		}

		typeDefinitions = append(typeDefinitions, td)
	}

	body := client.ClientWriteAuthorizationModelRequest{
		SchemaVersion:   authModel.SchemaVersion,
		TypeDefinitions: typeDefinitions,
	}

	response, err := fgaClient.WriteAuthorizationModel(ctx).Body(body).Execute()
	if err != nil {
		return "", err
	}

	return response.AuthorizationModelId, nil
}

// readAllTuples reads all tuples from the store
func (tc *TestContainer) readAllTuples(ctx context.Context, storeID string) ([]fgaSdk.Tuple, error) {
	configuration := &client.ClientConfiguration{
		ApiUrl:  tc.apiURL,
		StoreId: storeID,
	}

	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return nil, err
	}

	body := client.ClientReadRequest{}

	response, err := fgaClient.Read(ctx).Body(body).Execute()
	if err != nil {
		return nil, err
	}

	return response.Tuples, nil
}

func TestIntegration_UserLifecycle(t *testing.T) {
	ctx := context.Background()

	// Setup OpenFGA container
	container, err := setupOpenFGAContainer(ctx)
	require.NoError(t, err)
	defer container.Close()

	// Create store and model
	storeID, err := container.createTestStore(ctx, "user-lifecycle-test")
	require.NoError(t, err)

	modelID, err := container.createTestModel(ctx, storeID)
	require.NoError(t, err)

	// Load user mappings configuration
	userConfig, err := config.LoadMappingConfig("../../configs/user-mappings.yaml")
	require.NoError(t, err)

	// Create mapping engine
	engine := NewMappingEngine(container.apiURL, storeID, modelID)

	t.Run("Create User", func(t *testing.T) {
		// User creation event
		createEvent := map[string]interface{}{
			"type": "user.created",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id":        "auth0|test-user-1",
					"email":          "test@example.com",
					"email_verified": true,
					"phone_verified": false,
					"blocked":        false,
				},
			},
		}

		err = engine.ProcessEvent(ctx, createEvent, userConfig)
		assert.NoError(t, err)

		// Verify tuples were created
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 1) // Only email_verified should be created

		assert.Equal(t, "user:auth0|test-user-1", tuples[0].Key.User)
		assert.Equal(t, "email_verified", tuples[0].Key.Relation)
		assert.Equal(t, "user:auth0|test-user-1", tuples[0].Key.Object)
	})

	t.Run("Update User", func(t *testing.T) {
		// User update event - phone now verified, user blocked
		updateEvent := map[string]interface{}{
			"type": "user.updated",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id":        "auth0|test-user-1",
					"email_verified": true,
					"phone_verified": true, // Now verified
					"blocked":        true, // Now blocked
					"app_metadata": map[string]interface{}{
						"manager": "auth0|manager-1", // Added manager
					},
				},
			},
		}

		err = engine.ProcessEvent(ctx, updateEvent, userConfig)
		assert.NoError(t, err)

		// Verify tuples were updated
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 4) // email_verified, phone_verified, blocked, manager

		relations := make(map[string]string)
		for _, tuple := range tuples {
			relations[tuple.Key.Relation] = tuple.Key.Object
		}

		assert.Equal(t, "user:auth0|test-user-1", relations["email_verified"])
		assert.Equal(t, "user:auth0|test-user-1", relations["phone_verified"])
		assert.Equal(t, "user:auth0|test-user-1", relations["blocked"])
		assert.Equal(t, "user:auth0|manager-1", relations["manager"])
	})

	t.Run("Update User - Remove Relations", func(t *testing.T) {
		// User update event - remove some relations
		updateEvent := map[string]interface{}{
			"type": "user.updated",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id":        "auth0|test-user-1",
					"email_verified": true,
					"phone_verified": false, // No longer verified
					"blocked":        false, // No longer blocked
					// manager removed from app_metadata
				},
			},
		}

		err = engine.ProcessEvent(ctx, updateEvent, userConfig)
		assert.NoError(t, err)

		// Verify tuples were updated
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 1) // Only email_verified should remain

		assert.Equal(t, "user:auth0|test-user-1", tuples[0].Key.User)
		assert.Equal(t, "email_verified", tuples[0].Key.Relation)
		assert.Equal(t, "user:auth0|test-user-1", tuples[0].Key.Object)
	})

	t.Run("Delete User", func(t *testing.T) {
		// User deletion event
		deleteEvent := map[string]interface{}{
			"type": "user.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id": "auth0|test-user-1",
				},
			},
		}

		err = engine.ProcessEvent(ctx, deleteEvent, userConfig)
		assert.NoError(t, err)

		// Verify all tuples were deleted
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 0)
	})
}

func TestIntegration_OrganizationManagement(t *testing.T) {
	ctx := context.Background()

	// Setup OpenFGA container
	container, err := setupOpenFGAContainer(ctx)
	require.NoError(t, err)
	defer container.Close()

	// Create store and model
	storeID, err := container.createTestStore(ctx, "organization-test")
	require.NoError(t, err)

	modelID, err := container.createTestModel(ctx, storeID)
	require.NoError(t, err)

	// Load organization mappings configuration
	orgConfig, err := config.LoadMappingConfig("../../configs/organization-mappings.yaml")
	require.NoError(t, err)

	// Create mapping engine
	engine := NewMappingEngine(container.apiURL, storeID, modelID)

	t.Run("Create Organization", func(t *testing.T) {
		// Organization creation event
		createEvent := map[string]interface{}{
			"type": "organization.created",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id": "org_test_123",
					"metadata": map[string]interface{}{
						"external_org_id": "ext_org_999",
						"tier":            "premium",
					},
				},
			},
		}

		err = engine.ProcessEvent(ctx, createEvent, orgConfig)
		assert.NoError(t, err)

		// Verify tuples were created
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 2) // external_org and has_tier

		relations := make(map[string]string)
		for _, tuple := range tuples {
			relations[tuple.Key.Relation] = tuple.Key.Object
		}

		assert.Equal(t, "organization:org_test_123", relations["external_org"])
		assert.Equal(t, "tier:premium", relations["has_tier"])
	})

	t.Run("Update Organization", func(t *testing.T) {
		// Organization update event - change tier, remove external_org_id
		updateEvent := map[string]interface{}{
			"type": "organization.updated",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id": "org_test_123",
					"metadata": map[string]interface{}{
						"tier": "enterprise", // Changed tier
						// external_org_id removed
					},
				},
			},
		}

		err = engine.ProcessEvent(ctx, updateEvent, orgConfig)
		assert.NoError(t, err)

		// Verify tuples were updated
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 1) // Only has_tier should remain

		assert.Equal(t, "organization:org_test_123", tuples[0].Key.User)
		assert.Equal(t, "has_tier", tuples[0].Key.Relation)
		assert.Equal(t, "tier:enterprise", tuples[0].Key.Object)
	})

	t.Run("Delete Organization", func(t *testing.T) {
		// Organization deletion event
		deleteEvent := map[string]interface{}{
			"type": "organization.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id": "org_test_123",
				},
			},
		}

		err = engine.ProcessEvent(ctx, deleteEvent, orgConfig)
		assert.NoError(t, err)

		// Verify all tuples were deleted
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 0)
	})
}

func TestIntegration_OrganizationMembership(t *testing.T) {
	ctx := context.Background()

	// Setup OpenFGA container
	container, err := setupOpenFGAContainer(ctx)
	require.NoError(t, err)
	defer container.Close()

	// Create store and model
	storeID, err := container.createTestStore(ctx, "membership-test")
	require.NoError(t, err)

	modelID, err := container.createTestModel(ctx, storeID)
	require.NoError(t, err)

	// Load organization member mappings configuration
	memberConfig, err := config.LoadMappingConfig("../../configs/organization-member-mappings.yaml")
	require.NoError(t, err)

	// Create mapping engine
	engine := NewMappingEngine(container.apiURL, storeID, modelID)

	t.Run("Add Organization Member", func(t *testing.T) {
		// Member addition event
		addEvent := map[string]interface{}{
			"type": "organization.member.added",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|member-1",
					},
					"organization": map[string]interface{}{
						"id": "org_company_abc",
					},
				},
			},
		}

		err = engine.ProcessEvent(ctx, addEvent, memberConfig)
		assert.NoError(t, err)

		// Verify member tuple was created
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 1)

		assert.Equal(t, "user:auth0|member-1", tuples[0].Key.User)
		assert.Equal(t, "member", tuples[0].Key.Relation)
		assert.Equal(t, "organization:org_company_abc", tuples[0].Key.Object)
	})

	t.Run("Add Multiple Members", func(t *testing.T) {
		// Add another member
		addEvent := map[string]interface{}{
			"type": "organization.member.added",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|member-2",
					},
					"organization": map[string]interface{}{
						"id": "org_company_abc",
					},
				},
			},
		}

		err = engine.ProcessEvent(ctx, addEvent, memberConfig)
		assert.NoError(t, err)

		// Verify both member tuples exist
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 2)

		users := make(map[string]bool)
		for _, tuple := range tuples {
			users[tuple.Key.User] = true
			assert.Equal(t, "member", tuple.Key.Relation)
			assert.Equal(t, "organization:org_company_abc", tuple.Key.Object)
		}

		assert.True(t, users["user:auth0|member-1"])
		assert.True(t, users["user:auth0|member-2"])
	})

	t.Run("Delete Organization Member", func(t *testing.T) {
		// Member deletion event
		deleteEvent := map[string]interface{}{
			"type": "organization.member.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|member-1",
					},
					"organization": map[string]interface{}{
						"id": "org_company_abc",
					},
				},
			},
		}

		err = engine.ProcessEvent(ctx, deleteEvent, memberConfig)
		assert.NoError(t, err)

		// Verify only one member remains
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 1)

		assert.Equal(t, "user:auth0|member-2", tuples[0].Key.User)
		assert.Equal(t, "member", tuples[0].Key.Relation)
		assert.Equal(t, "organization:org_company_abc", tuples[0].Key.Object)
	})
}

func TestIntegration_RoleAssignments(t *testing.T) {
	ctx := context.Background()

	// Setup OpenFGA container
	container, err := setupOpenFGAContainer(ctx)
	require.NoError(t, err)
	defer container.Close()

	// Create store and model
	storeID, err := container.createTestStore(ctx, "role-assignment-test")
	require.NoError(t, err)

	modelID, err := container.createTestModel(ctx, storeID)
	require.NoError(t, err)

	// Load organization role mappings configuration
	roleConfig, err := config.LoadMappingConfig("../../configs/organization-role-mappings.yaml")
	require.NoError(t, err)

	// Create mapping engine
	engine := NewMappingEngine(container.apiURL, storeID, modelID)

	t.Run("Assign Role", func(t *testing.T) {
		// Role assignment event
		assignEvent := map[string]interface{}{
			"type": "organization.member.role.assigned",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|user-123",
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

		err = engine.ProcessEvent(ctx, assignEvent, roleConfig)
		assert.NoError(t, err)

		// Verify role tuple was created
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 1)

		assert.Equal(t, "user:auth0|user-123", tuples[0].Key.User)
		assert.Equal(t, "is_role", tuples[0].Key.Relation)
		assert.Equal(t, "role:admin|organization|org_company_abc", tuples[0].Key.Object)
	})

	t.Run("Assign Multiple Roles", func(t *testing.T) {
		// Assign another role to the same user
		assignEvent := map[string]interface{}{
			"type": "organization.member.role.assigned",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|user-123",
					},
					"role": map[string]interface{}{
						"id": "editor",
					},
					"organization": map[string]interface{}{
						"id": "org_company_abc",
					},
				},
			},
		}

		err = engine.ProcessEvent(ctx, assignEvent, roleConfig)
		assert.NoError(t, err)

		// Verify both role tuples exist
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 2)

		objects := make(map[string]bool)
		for _, tuple := range tuples {
			objects[tuple.Key.Object] = true
			assert.Equal(t, "user:auth0|user-123", tuple.Key.User)
			assert.Equal(t, "is_role", tuple.Key.Relation)
		}

		assert.True(t, objects["role:admin|organization|org_company_abc"])
		assert.True(t, objects["role:editor|organization|org_company_abc"])
	})

	t.Run("Delete Role", func(t *testing.T) {
		// Role deletion event
		deleteEvent := map[string]interface{}{
			"type": "organization.member.role.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|user-123",
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

		err = engine.ProcessEvent(ctx, deleteEvent, roleConfig)
		assert.NoError(t, err)

		// Verify only editor role remains
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 1)

		assert.Equal(t, "user:auth0|user-123", tuples[0].Key.User)
		assert.Equal(t, "is_role", tuples[0].Key.Relation)
		assert.Equal(t, "role:editor|organization|org_company_abc", tuples[0].Key.Object)
	})
}

func TestIntegration_MultiConfiguration(t *testing.T) {
	ctx := context.Background()

	// Setup OpenFGA container
	container, err := setupOpenFGAContainer(ctx)
	require.NoError(t, err)
	defer container.Close()

	// Create store and model
	storeID, err := container.createTestStore(ctx, "multi-config-test")
	require.NoError(t, err)

	modelID, err := container.createTestModel(ctx, storeID)
	require.NoError(t, err)

	// Load all mapping configurations
	configPaths := []string{
		"../../configs/user-mappings.yaml",
		"../../configs/organization-mappings.yaml",
		"../../configs/organization-member-mappings.yaml",
		"../../configs/organization-role-mappings.yaml",
	}

	configs, err := config.LoadMappingConfigs(configPaths)
	require.NoError(t, err)

	// Create multi-config processor
	processor := NewMultiConfigProcessor(container.apiURL, storeID, modelID, configs)

	t.Run("Complex Scenario", func(t *testing.T) {
		// Step 1: Create user
		userCreateEvent := map[string]interface{}{
			"type": "user.created",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id":        "auth0|complex-user",
					"email_verified": true,
					"phone_verified": true,
					"blocked":        false,
				},
			},
		}

		err = processor.ProcessEvent(ctx, userCreateEvent)
		assert.NoError(t, err)

		// Step 2: Create organization
		orgCreateEvent := map[string]interface{}{
			"type": "organization.created",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id": "org_complex_test",
					"metadata": map[string]interface{}{
						"tier": "enterprise",
					},
				},
			},
		}

		err = processor.ProcessEvent(ctx, orgCreateEvent)
		assert.NoError(t, err)

		// Step 3: Add user to organization
		memberAddEvent := map[string]interface{}{
			"type": "organization.member.added",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|complex-user",
					},
					"organization": map[string]interface{}{
						"id": "org_complex_test",
					},
				},
			},
		}

		err = processor.ProcessEvent(ctx, memberAddEvent)
		assert.NoError(t, err)

		// Step 4: Assign role
		roleAssignEvent := map[string]interface{}{
			"type": "organization.member.role.assigned",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|complex-user",
					},
					"role": map[string]interface{}{
						"id": "admin",
					},
					"organization": map[string]interface{}{
						"id": "org_complex_test",
					},
				},
			},
		}

		err = processor.ProcessEvent(ctx, roleAssignEvent)
		assert.NoError(t, err)

		// Verify all tuples were created correctly
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 5) // email_verified, phone_verified, has_tier, member, is_role

		// Organize tuples by relation for verification
		relationCounts := make(map[string]int)
		userRelations := make(map[string]string)
		orgRelations := make(map[string]string)

		for _, tuple := range tuples {
			relationCounts[tuple.Key.Relation]++

			if tuple.Key.User == "user:auth0|complex-user" {
				userRelations[tuple.Key.Relation] = tuple.Key.Object
			} else if tuple.Key.User == "organization:org_complex_test" {
				orgRelations[tuple.Key.Relation] = tuple.Key.Object
			}
		}

		// Verify user relations
		assert.Equal(t, "user:auth0|complex-user", userRelations["email_verified"])
		assert.Equal(t, "user:auth0|complex-user", userRelations["phone_verified"])
		assert.Equal(t, "organization:org_complex_test", userRelations["member"])
		assert.Equal(t, "role:admin|organization|org_complex_test", userRelations["is_role"])

		// Verify organization relations
		assert.Equal(t, "tier:enterprise", orgRelations["has_tier"])

		// Verify relation counts
		assert.Equal(t, 1, relationCounts["email_verified"])
		assert.Equal(t, 1, relationCounts["phone_verified"])
		assert.Equal(t, 1, relationCounts["has_tier"])
		assert.Equal(t, 1, relationCounts["member"])
		assert.Equal(t, 1, relationCounts["is_role"])
	})

	t.Run("Cleanup Scenario", func(t *testing.T) {
		// Delete role assignment
		roleDeleteEvent := map[string]interface{}{
			"type": "organization.member.role.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|complex-user",
					},
					"role": map[string]interface{}{
						"id": "admin",
					},
					"organization": map[string]interface{}{
						"id": "org_complex_test",
					},
				},
			},
		}

		err = processor.ProcessEvent(ctx, roleDeleteEvent)
		assert.NoError(t, err)

		// Remove member from organization
		memberDeleteEvent := map[string]interface{}{
			"type": "organization.member.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user": map[string]interface{}{
						"user_id": "auth0|complex-user",
					},
					"organization": map[string]interface{}{
						"id": "org_complex_test",
					},
				},
			},
		}

		err = processor.ProcessEvent(ctx, memberDeleteEvent)
		assert.NoError(t, err)

		// Delete organization
		orgDeleteEvent := map[string]interface{}{
			"type": "organization.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id": "org_complex_test",
				},
			},
		}

		err = processor.ProcessEvent(ctx, orgDeleteEvent)
		assert.NoError(t, err)

		// Delete user
		userDeleteEvent := map[string]interface{}{
			"type": "user.deleted",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id": "auth0|complex-user",
				},
			},
		}

		err = processor.ProcessEvent(ctx, userDeleteEvent)
		assert.NoError(t, err)

		// Verify all tuples were deleted
		tuples, err := container.readAllTuples(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, tuples, 0)
	})
}

func TestIntegration_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Setup OpenFGA container
	container, err := setupOpenFGAContainer(ctx)
	require.NoError(t, err)
	defer container.Close()

	// Create store and model
	storeID, err := container.createTestStore(ctx, "error-handling-test")
	require.NoError(t, err)

	modelID, err := container.createTestModel(ctx, storeID)
	require.NoError(t, err)

	// Create engine
	engine := NewMappingEngine(container.apiURL, storeID, modelID)

	t.Run("Invalid Event Type", func(t *testing.T) {
		config := &types.MappingConfig{
			Events: []types.EventMapping{
				{Type: "user.created", Action: "create"},
			},
			Mappings: []types.TupleMapping{},
		}

		// Event with unknown type
		event := map[string]interface{}{
			"type": "unknown.event.type",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id": "auth0|test",
				},
			},
		}

		err = engine.ProcessEvent(ctx, event, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no action found for event type")
	})

	t.Run("Invalid Template", func(t *testing.T) {
		config := &types.MappingConfig{
			Events: []types.EventMapping{
				{Type: "user.created", Action: "create"},
			},
			Mappings: []types.TupleMapping{
				{
					Tuple: types.TupleDefinition{
						User:     "user:{{ .invalid.template.syntax",
						Relation: "test",
						Object:   "object:test",
					},
				},
			},
		}

		event := map[string]interface{}{
			"type": "user.created",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id": "auth0|test",
				},
			},
		}

		err = engine.ProcessEvent(ctx, event, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template")
	})

	t.Run("Invalid Condition", func(t *testing.T) {
		config := &types.MappingConfig{
			Events: []types.EventMapping{
				{Type: "user.created", Action: "create"},
			},
			Mappings: []types.TupleMapping{
				{
					Condition: "invalid condition syntax !!!",
					Tuple: types.TupleDefinition{
						User:     "user:test",
						Relation: "test",
						Object:   "object:test",
					},
				},
			},
		}

		event := map[string]interface{}{
			"type": "user.created",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"user_id": "auth0|test",
				},
			},
		}

		err = engine.ProcessEvent(ctx, event, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition")
	})
}
