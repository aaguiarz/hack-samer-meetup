# Auth0 to OpenFGA Webhook Service

A Go-based webhook service that receives Auth0 events and maps them to OpenFGA authorization tuples using a configurable mapping engine.

## Features

- **HTTP Webhook Server**: Receives Auth0 webhook events via HTTP POST
- **Configurable OpenFGA Integration**: Supports multiple authentication methods (none, client credentials, shared secret)
- **Event Mapping Engine**: Maps Auth0 events to OpenFGA tuples using YAML configuration files
- **Signature Verification**: Validates Auth0 webhook signatures for security
- **Health Checks**: Built-in health check endpoint
- **Graceful Shutdown**: Handles shutdown signals gracefully
- **Structured Logging**: Comprehensive request/response logging
- **Error Recovery**: Panic recovery middleware

## Quick Start

### 1. Build the Service

```bash
go build -o webhook-service ./cmd/webhook-service
```

### 2. Configure Environment Variables

```bash
# OpenFGA Configuration
export OPENFGA_API_URL="http://localhost:8080"
export OPENFGA_STORE_ID="your-store-id"
export OPENFGA_AUTH_METHOD="none"  # or "client_credentials" or "shared_secret"

# For client credentials authentication:
# export OPENFGA_CLIENT_ID="your-client-id"
# export OPENFGA_CLIENT_SECRET="your-client-secret"
# export OPENFGA_AUDIENCE="https://your-openfga-api.com"
# export OPENFGA_ISSUER="https://your-auth-provider.com"

# For shared secret authentication:
# export OPENFGA_SHARED_SECRET="your-shared-secret"

# Auth0 Webhook Configuration
export AUTH0_WEBHOOK_SECRET="your-webhook-secret"

# Server Configuration (optional)
export PORT="8080"
export HOST="0.0.0.0"
```

### 3. Run the Service

```bash
./webhook-service
```

The service will start on `http://localhost:8080` by default.

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PORT` | HTTP server port | `8080` | No |
| `HOST` | HTTP server host | `0.0.0.0` | No |
| `OPENFGA_API_URL` | OpenFGA API URL | `http://localhost:8080` | No |
| `OPENFGA_STORE_ID` | OpenFGA store ID | - | Yes |
| `OPENFGA_MODEL_FILE` | Authorization model file path | `configs/model.json` | No |
| `OPENFGA_AUTH_METHOD` | Authentication method | `none` | No |
| `OPENFGA_CLIENT_ID` | Client ID for client credentials | - | If using client_credentials |
| `OPENFGA_CLIENT_SECRET` | Client secret for client credentials | - | If using client_credentials |
| `OPENFGA_AUDIENCE` | Audience for client credentials token | - | If using client_credentials |
| `OPENFGA_ISSUER` | Token issuer URL for client credentials | - | If using client_credentials |
| `OPENFGA_SHARED_SECRET` | Shared secret for API token auth | - | If using shared_secret |
| `AUTH0_WEBHOOK_SECRET` | Auth0 webhook secret for signature verification | - | Recommended |
| `AUTH0_VERIFY_SIGNATURE` | Enable signature verification | `true` | No |

### OpenFGA Authentication Methods

#### None (Development)
```bash
export OPENFGA_AUTH_METHOD="none"
```

#### Client Credentials (Production)
```bash
export OPENFGA_AUTH_METHOD="client_credentials"
export OPENFGA_CLIENT_ID="your-client-id"
export OPENFGA_CLIENT_SECRET="your-client-secret"
export OPENFGA_AUDIENCE="https://your-openfga-api.com"      # Optional
export OPENFGA_ISSUER="https://your-auth-provider.com"     # Optional
```

**Configuration Notes:**
- `OPENFGA_AUDIENCE`: The intended audience for the OAuth2 token. This should match the identifier of your OpenFGA API resource server.
- `OPENFGA_ISSUER`: The URL of the authorization server that will issue the tokens. Required if your OAuth2 provider uses a non-standard issuer URL.

#### Shared Secret (API Token)
```bash
export OPENFGA_AUTH_METHOD="shared_secret"
export OPENFGA_SHARED_SECRET="your-api-token"
```

### Mapping Configuration Files

The service uses YAML configuration files to map Auth0 events to OpenFGA tuples:

- `configs/user-mappings.yaml` - User lifecycle events
- `configs/organization-mappings.yaml` - Organization lifecycle events  
- `configs/organization-member-mappings.yaml` - Organization membership events
- `configs/organization-role-mappings.yaml` - Role assignment events

## API Endpoints

### Health Check
```
GET /health
```

Returns service health status:
```json
{
  "status": "healthy",
  "timestamp": "2023-06-26T12:00:00Z",
  "service": "auth0-openfga-webhook"
}
```

### Auth0 Webhook
```
POST /webhook/auth0
```

Receives Auth0 webhook events. Expects JSON payload with Auth0 event structure.

**Headers:**
- `Content-Type: application/json`
- `X-Hub-Signature-256: sha256=<signature>` (if signature verification enabled)

**Response:**
```json
{
  "status": "processed",
  "timestamp": "2023-06-26T12:00:00Z",
  "event_type": "user.created"
}
```

## Testing with curl

Here are examples of how to send Auth0 events to the webhook using curl:

### User Created Event
```bash
curl -X POST http://localhost:8080/webhook/auth0 \
  -H "Content-Type: application/json" \
  -d '{
    "type": "user.created",
    "data": {
      "object": {
        "user_id": "auth0|user123",
        "email": "user@example.com",
        "email_verified": true,
        "phone_verified": false
      }
    }
  }'
```

### Organization Created Event
```bash
curl -X POST http://localhost:8080/webhook/auth0 \
  -H "Content-Type: application/json" \
  -d '{
    "type": "organization.created",
    "data": {
      "object": {
        "id": "org_abc123",
        "name": "Example Corp",
        "metadata": {
          "external_org_id": "ext-org-456",
          "tier": "enterprise"
        }
      }
    }
  }'
```

### Organization Member Added Event
```bash
curl -X POST http://localhost:8080/webhook/auth0 \
  -H "Content-Type: application/json" \
  -d '{
    "type": "organization.member.added",
    "data": {
      "object": {
        "user": {
          "user_id": "auth0|user123"
        },
        "organization": {
          "id": "org_abc123"
        }
      }
    }
  }'
```

### Role Assignment Event
```bash
curl -X POST http://localhost:8080/webhook/auth0 \
  -H "Content-Type: application/json" \
  -d '{
    "type": "organization.member.role.assigned",
    "data": {
      "object": {
        "user": {
          "user_id": "auth0|user123"
        },
        "organization": {
          "id": "org_abc123"
        },
        "role": {
          "id": "admin"
        }
      }
    }
  }'
```

### Testing with Signature Verification

If you have signature verification enabled, you need to include the `X-Hub-Signature-256` header:

```bash
# First, generate the signature (example using openssl)
PAYLOAD='{"type":"user.created","data":{"object":{"user_id":"auth0|user123"}}}'
SECRET="your-webhook-secret"
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" -binary | xxd -p -c 256)

# Then send the request with the signature
curl -X POST http://localhost:8080/webhook/auth0 \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=$SIGNATURE" \
  -d "$PAYLOAD"
```

### Expected Responses

**Success Response:**
```json
{
  "status": "processed",
  "timestamp": "2025-06-26T12:00:00Z",
  "event_type": "user.created"
}
```

**Error Response (Invalid JSON):**
```
HTTP 400 Bad Request
Invalid JSON
```

**Error Response (Missing Event Type):**
```
HTTP 500 Internal Server Error
Failed to process event
```

## Docker Deployment

### Build Docker Image

```bash
docker build -f Dockerfile.webhook -t auth0-openfga-webhook .
```

### Run with Docker

```bash
docker run -d \
  --name auth0-openfga-webhook \
  -p 8080:8080 \
  -e OPENFGA_API_URL="http://openfga:8080" \
  -e OPENFGA_STORE_ID="your-store-id" \
  -e AUTH0_WEBHOOK_SECRET="your-webhook-secret" \
  auth0-openfga-webhook
```

### Docker Compose Example

```yaml
version: '3.8'
services:
  webhook:
    build:
      context: .
      dockerfile: Dockerfile.webhook
    ports:
      - "8080:8080"
    environment:
      - OPENFGA_API_URL=http://openfga:8080
      - OPENFGA_STORE_ID=your-store-id
      - AUTH0_WEBHOOK_SECRET=your-webhook-secret
    depends_on:
      - openfga
    
  openfga:
    image: openfga/openfga:latest
    ports:
      - "8080:8080"
    command: run --playground-enabled=false
```

## Auth0 Webhook Setup

1. **Create a Webhook** in your Auth0 Dashboard
2. **Set the Endpoint URL** to your service: `https://your-domain.com/webhook/auth0`
3. **Configure Events** to include the events you want to map:
   - `user.created`
   - `user.updated` 
   - `user.deleted`
   - `organization.created`
   - `organization.updated`
   - `organization.deleted`
   - `organization.member.added`
   - `organization.member.deleted`
   - `organization.member.role.assigned`
   - `organization.member.role.deleted`
4. **Set the Secret** and configure the same value in `AUTH0_WEBHOOK_SECRET`

## Event Processing Flow

1. **Webhook Reception**: Service receives Auth0 webhook event
2. **Signature Verification**: Validates request signature (if enabled)
3. **Event Parsing**: Parses JSON payload
4. **Mapping Selection**: Selects appropriate mapping configuration based on event type
5. **Tuple Generation**: Processes event through mapping engine to generate OpenFGA tuples
6. **OpenFGA Update**: Writes/updates/deletes tuples in OpenFGA
7. **Response**: Returns success/error response

## Monitoring and Logging

The service provides structured logging for:
- HTTP requests and responses
- Event processing
- OpenFGA operations
- Errors and panics

Example log output:
```
2023/06/26 12:00:00 POST /webhook/auth0 200 45.123ms
2023/06/26 12:00:00 Processing event: user.created
2023/06/26 12:00:00 Created 2 tuples for user: auth0|user123
```

## Error Handling

The service handles various error conditions:
- Invalid JSON payloads
- Missing event types
- Mapping configuration errors
- OpenFGA connection failures
- Template processing errors

All errors are logged with context and appropriate HTTP status codes are returned.

## Security Considerations

1. **Enable Signature Verification**: Always use `AUTH0_WEBHOOK_SECRET` in production
2. **Use HTTPS**: Deploy behind HTTPS proxy/load balancer
3. **Network Security**: Restrict access to webhook endpoint
4. **OpenFGA Authentication**: Use client credentials or shared secret authentication
5. **Environment Variables**: Store secrets in environment variables, not configuration files

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run integration tests specifically
go test -v ./internal/engine/ -run "TestIntegration_"
```

### Code Structure

```
├── cmd/webhook-service/     # Main application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── engine/              # Mapping engine logic
│   ├── service/             # HTTP service implementation
│   └── types/               # Type definitions
├── configs/                 # Configuration files
└── Dockerfile.webhook       # Docker build configuration
```

## Troubleshooting

### Common Issues

1. **OpenFGA Connection Failed**
   - Check `OPENFGA_API_URL` is correct
   - Verify OpenFGA is running and accessible
   - Check authentication credentials

2. **Webhook Signature Verification Failed**
   - Verify `AUTH0_WEBHOOK_SECRET` matches Auth0 configuration
   - Check that Auth0 is sending `X-Hub-Signature-256` header

3. **Event Not Processed**
   - Check that event type is configured in mapping files
   - Verify mapping configuration syntax
   - Check service logs for template processing errors

4. **Store ID Not Found**
   - Ensure `OPENFGA_STORE_ID` is set correctly
   - Verify store exists in OpenFGA instance

### Debug Mode

Enable verbose logging by setting log level:
```bash
export LOG_LEVEL=debug
```

## Production Configuration Examples

### Complete Auth0 + OpenFGA Cloud Setup
```bash
# Server Configuration
export PORT="8080"
export HOST="0.0.0.0"

# OpenFGA Cloud Configuration
export OPENFGA_API_URL="https://api.us1.fga.dev"
export OPENFGA_STORE_ID="01HXYZ123456789ABCDEF"
export OPENFGA_AUTH_METHOD="client_credentials"
export OPENFGA_CLIENT_ID="your-openfga-client-id"
export OPENFGA_CLIENT_SECRET="your-openfga-client-secret"
export OPENFGA_AUDIENCE="https://api.us1.fga.dev"
export OPENFGA_ISSUER="https://auth.us1.fga.dev"

# Auth0 Configuration
export AUTH0_WEBHOOK_SECRET="your-strong-webhook-secret"
export AUTH0_VERIFY_SIGNATURE="true"

# Start the service
./webhook-service
```

### Self-Hosted OpenFGA with OAuth2
```bash
# OpenFGA Self-Hosted with OAuth2
export OPENFGA_API_URL="https://openfga.yourcompany.com"
export OPENFGA_STORE_ID="your-store-id"
export OPENFGA_AUTH_METHOD="client_credentials"
export OPENFGA_CLIENT_ID="webhook-service"
export OPENFGA_CLIENT_SECRET="your-oauth2-client-secret"
export OPENFGA_AUDIENCE="https://openfga.yourcompany.com"
export OPENFGA_ISSUER="https://auth.yourcompany.com"

# Auth0 Configuration
export AUTH0_WEBHOOK_SECRET="your-webhook-secret"

# Start the service
./webhook-service
```
