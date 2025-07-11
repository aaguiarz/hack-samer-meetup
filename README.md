# Auth0 to OpenFGA Mapping Engine

A Go-based mapping engine that processes Auth0 events and maps them to OpenFGA tuples based on configurable rules.

## Features

- **Conditional Mapping**: Map Auth0 events to OpenFGA tuples based on field conditions
- **Template Processing**: Use Go templates to dynamically construct tuple values
- **Multiple Actions**: Support for create, update, and delete operations
- **YAML Configuration**: Define mappings in easy-to-read YAML files
- **Multi-Config Support**: Process events against multiple mapping configurations
- **Comprehensive Testing**: Full test coverage with testcontainers for integration testing

## Tools

This project provides multiple tools for different use cases:

### 1. Event Processor CLI
A command-line tool for batch processing Auth0 events from JSON files.

**Features:**
- Process Auth0 events from JSON files
- Dry-run mode to preview changes
- Detailed output showing tuple operations
- Support for multiple authentication methods
- Comprehensive error reporting

**Quick Start:**
```bash
# Build the CLI
go build -o bin/event-processor cmd/event-processor/main.go

# Process events in dry-run mode
./bin/event-processor -events examples/sample-events.json -dry-run -verbose

# Process events for real
./bin/event-processor -events examples/sample-events.json -store-id <store-id>
```

**Documentation:** [CLI Documentation](README-cli.md)

### 2. Webhook Service
A HTTP service that receives Auth0 webhooks and processes them in real-time.

**Features:**
- Real-time event processing via webhooks
- Signature verification for security
- Health check endpoints
- Configurable OpenFGA authentication
- Docker support

**Quick Start:**
```bash
# Build the webhook service
go build -o bin/webhook-service cmd/webhook-service/main.go

# Run the service
./bin/webhook-service -config configs/service.yaml
```

**Documentation:** [Webhook Service Documentation](README-webhook.md)

### 3. Mapping Engine Library
A Go library for programmatic event processing.

**Features:**
- Direct integration into Go applications
- Event processing with detailed results
- Support for custom OpenFGA clients
- Comprehensive error handling

**Example:**
```go
engine := engine.NewMappingEngine(apiURL, storeID, modelID)
result, err := engine.ProcessEventWithDetails(ctx, event, config)
```

**Documentation:** [Library Examples](examples/complete_example.go)

## Architecture

### Core Components

1. **MappingEngine**: Core engine that processes individual events
2. **MultiConfigProcessor**: Handles events across multiple mapping configurations
3. **Configuration Loader**: Loads mapping rules from YAML files
4. **Template Processor**: Processes Go templates in tuple definitions
5. **Condition Evaluator**: Evaluates expressions to determine if mappings should apply

### Event Processing Flow

```
Auth0 Event → Condition Evaluation → Template Processing → OpenFGA Operations
```

For each event:
1. Determine the action type (create/update/delete)
2. Evaluate mapping conditions against event data
3. Process templates to generate tuple values
4. Execute appropriate OpenFGA operations

## Configuration Format

### Event Mappings

Define which Auth0 events map to which actions:

```yaml
events:
  - type: user.created
    action: create
  - type: user.updated
    action: update
  - type: user.deleted
    action: delete
```

### Tuple Mappings

Define conditional mappings to OpenFGA tuples:

```yaml
mappings:
  # Map if email is verified
  - condition: "data.object.email_verified == true"
    tuple:
      user: "user:{{ .data.object.user_id }}"
      relation: "email_verified"
      object: "user:{{ .data.object.user_id }}"

  # Map manager relationship
  - condition: "data.object.app_metadata.manager != null"
    tuple:
      user: "user:{{ .data.object.user_id }}"
      relation: "manager"
      object: "user:{{ .data.object.app_metadata.manager }}"
```

## Action Types

### Create Actions

- Write all matching tuples to OpenFGA in a single operation
- Fail if tuples already exist

### Update Actions

- Compare current event state with existing OpenFGA tuples
- Add new tuples that should exist
- Remove tuples that should no longer exist
- Update tuples with different values

### Delete Actions

- Read all existing tuples for the user
- Delete all found tuples

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "mapping-engine/internal/engine"
    "mapping-engine/internal/types"
)

func main() {
    // Create mapping engine
    mappingEngine := engine.NewMappingEngine(
        "http://localhost:8080", // OpenFGA URL
        "store-id",              // Store ID
        "model-id",              // Model ID
    )

    // Define configuration
    config := &types.MappingConfig{
        Events: []types.EventMapping{
            {Type: "user.created", Action: "create"},
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
        },
    }

    // Process event
    event := map[string]interface{}{
        "type": "user.created",
        "data": map[string]interface{}{
            "object": map[string]interface{}{
                "user_id": "auth0|123456",
                "email_verified": true,
            },
        },
    }

    ctx := context.Background()
    err := mappingEngine.ProcessEvent(ctx, event, config)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Multi-Configuration Usage

```go
// Load configurations from YAML files
configs, err := config.LoadMappingConfigs([]string{
    "configs/user-mappings.yaml",
    "configs/organization-mappings.yaml",
})

// Create multi-config processor
processor := engine.NewMultiConfigProcessor(
    apiURL, storeID, modelID, configs,
)

// Process event against all applicable configurations
err = processor.ProcessEvent(ctx, event)
```

## Template Syntax

The engine uses Go templates to process tuple values. You can access any field from the Auth0 event:

```yaml
# Simple field access
user: "user:{{ .data.object.user_id }}"

# Nested field access
object: "user:{{ .data.object.app_metadata.manager }}"

# Complex object construction
object: "role:{{ .data.object.role.id }}#organization:{{ .data.object.organization.id }}"
```

## Condition Expressions

Conditions use the `expr` library for expression evaluation:

```yaml
# Boolean comparisons
condition: "data.object.email_verified == true"
condition: "data.object.blocked == false"

# Null checks
condition: "data.object.app_metadata.manager != null"

# String comparisons
condition: "data.object.connection == 'Username-Password-Authentication'"

# Complex expressions
condition: "data.object.email_verified == true && data.object.logins_count > 0"
```

## Example Configurations

### User Events

```yaml
# configs/user-mappings.yaml
events:
  - type: user.created
    action: create
  - type: user.updated
    action: update
  - type: user.deleted
    action: delete

mappings:
  - condition: "data.object.email_verified == true"
    tuple:
      user: "user:{{ .data.object.user_id }}"
      relation: "email_verified"
      object: "user:{{ .data.object.user_id }}"

  - condition: "data.object.blocked == true"
    tuple:
      user: "user:{{ .data.object.user_id }}"
      relation: "blocked"
      object: "user:{{ .data.object.user_id }}"
```

### Organization Events

```yaml
# configs/organization-mappings.yaml
events:
  - type: organization.created
    action: create
  - type: organization.updated
    action: update

mappings:
  - condition: "data.object.metadata.tier != null"
    tuple:
      user: "organization:{{ .data.object.id }}"
      relation: "has_tier"
      object: "tier:{{ .data.object.metadata.tier }}"
```

## Testing

The project includes comprehensive tests using testcontainers to run actual OpenFGA instances:

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test
go test -v ./internal/engine -run TestMappingEngine_ProcessCreateEvent
```

### Test Coverage

- Create, update, and delete operations
- Condition evaluation
- Template processing
- Error handling
- Integration with real OpenFGA server
- Complex scenarios with multiple event types

## Quick Demo

To see the mapping engine in action without setting up OpenFGA:

```bash
go run demo/main.go
```

This will show:
- How Auth0 events are parsed
- How mapping rules are evaluated
- How templates are processed to generate tuples
- What the resulting OpenFGA operations would be

## Building and Running

```bash
# Download dependencies
go mod tidy

# Run the main example
go run cmd/main.go

# Run the complete example
go run examples/complete_example.go

# Build the project
go build -o mapping-engine cmd/main.go
```

## Dependencies

- **OpenFGA Go SDK**: For OpenFGA operations
- **expr**: For condition evaluation
- **yaml.v3**: For YAML configuration parsing
- **testcontainers**: For integration testing
- **testify**: For test assertions

## Configuration Examples

The `configs/` directory contains example mapping configurations for different Auth0 event types:

- `user-mappings.yaml`: User lifecycle events
- `organization-mappings.yaml`: Organization events
- `organization-member-mappings.yaml`: Organization membership events
- `organization-role-mappings.yaml`: Role assignment events

## Error Handling

The engine provides detailed error messages for:

- Configuration parsing errors
- Template processing errors
- Condition evaluation errors
- OpenFGA operation failures
- Missing required fields

## Performance Considerations

- Batch operations when possible
- Efficient tuple comparison for updates
- Minimal OpenFGA API calls
- Parallel processing of multiple configurations

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

This project is licensed under the MIT License.
