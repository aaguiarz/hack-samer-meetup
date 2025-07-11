# Auth0 to OpenFGA Event Processor CLI

A command-line tool for processing Auth0 events and converting them to OpenFGA relationship tuples.

## Overview

The Event Processor CLI reads a JSON file containing Auth0 events, processes them according to configurable mapping rules, and writes the resulting relationship tuples to OpenFGA. It provides detailed output showing exactly which tuples are added or deleted for each event.

## Features

- **Batch Processing**: Process multiple Auth0 events from a JSON file
- **Dry Run Mode**: Preview what changes would be made without actually writing to OpenFGA
- **Detailed Output**: Shows exact tuples added/deleted for each event
- **Multiple Authentication Methods**: Supports various OpenFGA authentication methods
- **Configurable Mappings**: Uses YAML configuration files for different event types
- **Comprehensive Reporting**: Provides detailed summary of processing results

## Installation

Build the CLI tool:

```bash
go build -o bin/event-processor cmd/event-processor/main.go
```

## Usage

### Basic Usage

```bash
./bin/event-processor -events events.json -store-id <store-id>
```

### Dry Run Mode

Preview changes without making them:

```bash
./bin/event-processor -events events.json -store-id <store-id> -dry-run -verbose
```

### With Authentication

```bash
# Using OAuth2 Client Credentials
./bin/event-processor \
  -events events.json \
  -store-id <store-id> \
  -auth-method client_credentials \
  -client-id <client-id> \
  -client-secret <client-secret> \
  -audience <audience> \
  -issuer <issuer>

# Using Shared Secret
./bin/event-processor \
  -events events.json \
  -store-id <store-id> \
  -auth-method shared_secret \
  -shared-secret <secret>
```

## Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-events` | Path to JSON file containing Auth0 events | **Required** |
| `-store-id` | OpenFGA Store ID | **Required** |
| `-model-id` | OpenFGA Authorization Model ID | |
| `-openfga-url` | OpenFGA API URL | `http://localhost:8080` |
| `-model-file` | OpenFGA model file | `configs/model.json` |
| `-auth-method` | Authentication method (none, client_credentials, shared_secret) | `none` |
| `-client-id` | OAuth2 Client ID | |
| `-client-secret` | OAuth2 Client Secret | |
| `-audience` | OAuth2 audience | |
| `-issuer` | OAuth2 token issuer | |
| `-shared-secret` | Shared secret for API token auth | |
| `-dry-run` | Show what would be done without making changes | `false` |
| `-verbose` | Enable verbose output | `false` |
| `-user-mappings` | User mappings file | `configs/user-mappings.yaml` |
| `-org-mappings` | Organization mappings file | `configs/organization-mappings.yaml` |
| `-org-member-mappings` | Organization member mappings file | `configs/organization-member-mappings.yaml` |
| `-org-role-mappings` | Organization role mappings file | `configs/organization-role-mappings.yaml` |

## Event File Format

The events file should be a JSON array containing Auth0 events. Each event should have the following structure:

```json
[
  {
    "type": "user.created",
    "data": {
      "object": {
        "user_id": "auth0|user123",
        "email": "john.doe@example.com",
        "email_verified": true,
        "phone_verified": false,
        "blocked": false
      }
    },
    "created_at": "2024-01-15T10:30:00Z"
  },
  {
    "type": "organization.created",
    "data": {
      "object": {
        "id": "org_12345",
        "name": "Acme Corp",
        "metadata": {
          "tier": "enterprise"
        }
      }
    },
    "created_at": "2024-01-15T11:00:00Z"
  }
]
```

## Supported Event Types

| Event Type | Action | Description |
|------------|--------|-------------|
| `user.created` | create | Create user-related tuples |
| `user.updated` | update | Update user-related tuples |
| `user.deleted` | delete | Delete user-related tuples |
| `organization.created` | create | Create organization-related tuples |
| `organization.updated` | update | Update organization-related tuples |
| `organization.deleted` | delete | Delete organization-related tuples |
| `organization.member.added` | create | Create organization membership tuples |
| `organization.member.removed` | delete | Delete organization membership tuples |
| `organization.member.deleted` | delete | Delete organization membership tuples |
| `organization.member.role.assigned` | create | Create organization role tuples |
| `organization.member.role.deleted` | delete | Delete organization role tuples |

## Output Examples

### Verbose Mode

```
🚀 Auth0 to OpenFGA Event Processor
====================================
📁 Events file: examples/sample-events.json
🎯 OpenFGA URL: http://localhost:8080
🏪 Store ID: 01HXD8QZPQR1234567890
� Model ID: 01HXDB9MNPQR1234567890
�📊 Total events: 6
🔍 DRY RUN MODE - No changes will be made

[1/6] ✅ SUCCESS - user.created (274.125µs)
   📝 Tuples Added:
      + user:auth0|user123 email_verified user:auth0|user123

[2/6] ✅ SUCCESS - organization.created (137.083µs)
   📝 Tuples Added:
      + organization:org_12345 has_tier tier:enterprise

[3/6] ✅ SUCCESS - organization.member.removed (75.833µs)
   🗑️ Tuples Deleted:
      - user:auth0|user123 member organization:org_12345

📊 Processing Summary
====================
📈 Total Events: 6
✅ Successful: 6
❌ Failed: 0
📝 Total Tuples Added: 6
🗑️ Total Tuples Deleted: 1
⏱️ Total Duration: 974.833µs
📊 Average Duration: 162.472µs
```

### Summary Mode

```
🚀 Auth0 to OpenFGA Event Processor
====================================
📁 Events file: examples/sample-events.json
🎯 OpenFGA URL: http://localhost:8080
🏪 Store ID: 01HXD8QZPQR1234567890
📊 Total events: 6

[1/6] ✅ user.created (186.958µs)
[2/6] ✅ organization.created (115.458µs)
[3/6] ✅ organization.member.added (99.041µs)
[4/6] ❌ organization.unknown (50.123µs)
   Error: no mapping configuration found for event type: organization.unknown

📊 Processing Summary
====================
📈 Total Events: 6
✅ Successful: 5
❌ Failed: 1
📝 Total Tuples Added: 6
🗑️ Total Tuples Deleted: 1
```

## Environment Variables

You can also set configuration using environment variables:

- `OPENFGA_API_URL`: OpenFGA API URL
- `OPENFGA_STORE_ID`: OpenFGA Store ID
- `OPENFGA_MODEL_ID`: OpenFGA Authorization Model ID
- `OPENFGA_MODEL_FILE`: OpenFGA model file path
- `OPENFGA_AUTH_METHOD`: Authentication method
- `OPENFGA_CLIENT_ID`: OAuth2 Client ID
- `OPENFGA_CLIENT_SECRET`: OAuth2 Client Secret
- `OPENFGA_AUDIENCE`: OAuth2 audience
- `OPENFGA_ISSUER`: OAuth2 token issuer
- `OPENFGA_SHARED_SECRET`: Shared secret

## Examples

### Example 1: Basic Processing

```bash
# Process events and write to OpenFGA
./bin/event-processor \
  -events examples/sample-events.json \
  -store-id 01HXD8QZPQR1234567890 \
  -model-id 01HXDB9MNPQR1234567890
```

### Example 2: Dry Run with Verbose Output

```bash
# Preview changes without making them
./bin/event-processor \
  -events examples/sample-events.json \
  -store-id 01HXD8QZPQR1234567890 \
  -model-id 01HXDB9MNPQR1234567890 \
  -dry-run \
  -verbose
```

### Example 3: With Custom Mappings

```bash
# Use custom mapping files
./bin/event-processor \
  -events my-events.json \
  -store-id 01HXD8QZPQR1234567890 \
  -user-mappings custom/user-mappings.yaml \
  -org-mappings custom/org-mappings.yaml
```

### Example 4: Production Setup with Authentication

```bash
# Production setup with OAuth2
export OPENFGA_API_URL="https://api.fga.example.com"
export OPENFGA_STORE_ID="01HXD8QZPQR1234567890"
export OPENFGA_MODEL_ID="01HXDB9MNPQR1234567890"
export OPENFGA_CLIENT_ID="your-client-id"
export OPENFGA_CLIENT_SECRET="your-client-secret"
export OPENFGA_AUDIENCE="https://api.fga.example.com/"
export OPENFGA_ISSUER="https://auth.example.com/"

./bin/event-processor \
  -events production-events.json \
  -auth-method client_credentials
```

## Error Handling

The CLI provides detailed error information:

- **Configuration Errors**: Missing required parameters, invalid file paths
- **Event Processing Errors**: Malformed events, template processing failures
- **OpenFGA Errors**: API connection issues, authentication failures
- **Mapping Errors**: Unknown event types, condition evaluation failures

Failed events are reported in the summary with specific error messages.

## Integration

The CLI can be integrated into CI/CD pipelines, batch processing workflows, or used for:

- **Migration**: Bulk import of historical Auth0 events
- **Testing**: Validate mapping configurations with sample data
- **Monitoring**: Process event logs and verify tuple operations
- **Development**: Test changes to mapping configurations

## Related Tools

- [Webhook Service](README-webhook.md): Real-time event processing via webhooks
- [Mapping Engine](examples/complete_example.go): Programmatic API for event processing
