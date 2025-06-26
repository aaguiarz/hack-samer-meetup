package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"mapping-engine/internal/types"
)

func TestMappingEngine_EvaluateCondition(t *testing.T) {
	engine := &MappingEngine{}

	tests := []struct {
		name      string
		condition string
		event     map[string]interface{}
		expected  bool
		wantError bool
	}{
		{
			name:      "email verified true",
			condition: "data.object.email_verified == true",
			event: map[string]interface{}{
				"data": map[string]interface{}{
					"object": map[string]interface{}{
						"email_verified": true,
					},
				},
			},
			expected:  true,
			wantError: false,
		},
		{
			name:      "email verified false",
			condition: "data.object.email_verified == true",
			event: map[string]interface{}{
				"data": map[string]interface{}{
					"object": map[string]interface{}{
						"email_verified": false,
					},
				},
			},
			expected:  false,
			wantError: false,
		},
		{
			name:      "manager not null",
			condition: "data.object.metadata != nil && data.object.metadata.manager != nil",
			event: map[string]interface{}{
				"data": map[string]interface{}{
					"object": map[string]interface{}{
						"metadata": map[string]interface{}{
							"manager": "user123",
						},
					},
				},
			},
			expected:  true,
			wantError: false,
		},
		{
			name:      "manager is null",
			condition: "data.object.metadata != nil && data.object.metadata.manager != nil",
			event: map[string]interface{}{
				"data": map[string]interface{}{
					"object": map[string]interface{}{
						"metadata": map[string]interface{}{},
					},
				},
			},
			expected:  false,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.evaluateCondition(tt.condition, tt.event)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMappingEngine_ProcessTemplates(t *testing.T) {
	engine := &MappingEngine{}

	event := map[string]interface{}{
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user_id": "auth0|123456",
				"role": map[string]interface{}{
					"id": "admin",
				},
				"organization": map[string]interface{}{
					"id": "org_123",
				},
			},
		},
	}

	tests := []struct {
		name       string
		definition types.TupleDefinition
		expected   types.ProcessedTuple
	}{
		{
			name: "simple user template",
			definition: types.TupleDefinition{
				User:     "user:{{ .data.object.user_id }}",
				Relation: "email_verified",
				Object:   "user:{{ .data.object.user_id }}",
			},
			expected: types.ProcessedTuple{
				User:     "user:auth0|123456",
				Relation: "email_verified",
				Object:   "user:auth0|123456",
			},
		},
		{
			name: "complex nested template",
			definition: types.TupleDefinition{
				User:     "user:{{ .data.object.user_id }}",
				Relation: "is_role",
				Object:   "role:{{ .data.object.role.id }}#organization:{{ .data.object.organization.id }}",
			},
			expected: types.ProcessedTuple{
				User:     "user:auth0|123456",
				Relation: "is_role",
				Object:   "role:admin#organization:org_123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.processTemplates(tt.definition, event)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMappingEngine_EvaluateMappings(t *testing.T) {
	engine := &MappingEngine{}

	event := map[string]interface{}{
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"user_id":        "auth0|123456",
				"email_verified": true,
				"phone_verified": false,
			},
		},
	}

	mappings := []types.TupleMapping{
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
	}

	result, err := engine.evaluateMappings(event, mappings)
	assert.NoError(t, err)
	assert.Len(t, result, 1) // Only email_verified should match
	assert.Equal(t, "user:auth0|123456", result[0].User)
	assert.Equal(t, "email_verified", result[0].Relation)
	assert.Equal(t, "user:auth0|123456", result[0].Object)
}

func TestMappingEngine_CalculateTupleChanges(t *testing.T) {
	engine := &MappingEngine{}

	existing := []types.ProcessedTuple{
		{User: "user:123", Relation: "email_verified", Object: "user:123"},
		{User: "user:123", Relation: "blocked", Object: "user:123"},
	}

	new := []types.ProcessedTuple{
		{User: "user:123", Relation: "email_verified", Object: "user:123"},
		{User: "user:123", Relation: "phone_verified", Object: "user:123"},
	}

	toAdd, toDelete := engine.calculateTupleChanges(existing, new)

	assert.Len(t, toAdd, 1)
	assert.Equal(t, "phone_verified", toAdd[0].Relation)

	assert.Len(t, toDelete, 1)
	assert.Equal(t, "blocked", toDelete[0].Relation)
}
