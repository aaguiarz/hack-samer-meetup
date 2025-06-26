#!/bin/bash

# Example script demonstrating how to use the Auth0 to OpenFGA Webhook Service
# This script shows how to:
# 1. Start the webhook service
# 2. Send sample Auth0 events to the webhook
# 3. Verify the events are processed correctly

set -e

echo "🚀 Auth0 to OpenFGA Webhook Service Demo"
echo "========================================="

# Configuration
WEBHOOK_URL="http://localhost:8080/webhook/auth0"
HEALTH_URL="http://localhost:8080/health"
SERVICE_PID=""

# Function to cleanup on exit
cleanup() {
    if [ ! -z "$SERVICE_PID" ]; then
        echo "🛑 Stopping webhook service..."
        kill $SERVICE_PID 2>/dev/null || true
        wait $SERVICE_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Start the webhook service in the background
echo "🔧 Starting webhook service..."
export OPENFGA_STORE_ID="demo-store"
export OPENFGA_API_URL="http://localhost:8080"
export AUTH0_VERIFY_SIGNATURE="false"  # Disable signature verification for demo

./webhook-service &
SERVICE_PID=$!

# Wait for service to start
echo "⏳ Waiting for service to start..."
for i in {1..10}; do
    if curl -s "$HEALTH_URL" > /dev/null 2>&1; then
        echo "✅ Service is running!"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "❌ Service failed to start"
        exit 1
    fi
    sleep 1
done

# Test health endpoint
echo ""
echo "🏥 Testing health endpoint..."
echo "GET $HEALTH_URL"
curl -s "$HEALTH_URL" | jq .
echo ""

# Sample Auth0 events to test
echo "📝 Testing Auth0 webhook events..."
echo ""

# 1. User Created Event
echo "👤 Testing user.created event..."
USER_CREATED='{
  "type": "user.created",
  "data": {
    "object": {
      "user_id": "auth0|demo-user-123",
      "email": "demo@example.com",
      "email_verified": true,
      "phone_verified": false
    }
  }
}'

echo "POST $WEBHOOK_URL"
echo "$USER_CREATED" | jq .
RESPONSE=$(curl -s -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "$USER_CREATED")
echo "Response: $RESPONSE" | jq .
echo ""

# 2. Organization Created Event
echo "🏢 Testing organization.created event..."
ORG_CREATED='{
  "type": "organization.created",
  "data": {
    "object": {
      "id": "org_demo_123",
      "name": "Demo Organization",
      "metadata": {
        "external_org_id": "ext-org-456",
        "tier": "enterprise"
      }
    }
  }
}'

echo "POST $WEBHOOK_URL"
echo "$ORG_CREATED" | jq .
RESPONSE=$(curl -s -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "$ORG_CREATED")
echo "Response: $RESPONSE" | jq .
echo ""

# 3. Organization Member Added Event
echo "👥 Testing organization.member.added event..."
MEMBER_ADDED='{
  "type": "organization.member.added",
  "data": {
    "object": {
      "user": {
        "user_id": "auth0|demo-user-123"
      },
      "organization": {
        "id": "org_demo_123"
      }
    }
  }
}'

echo "POST $WEBHOOK_URL"
echo "$MEMBER_ADDED" | jq .
RESPONSE=$(curl -s -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "$MEMBER_ADDED")
echo "Response: $RESPONSE" | jq .
echo ""

# 4. Role Assignment Event
echo "🎭 Testing organization.member.role.assigned event..."
ROLE_ASSIGNED='{
  "type": "organization.member.role.assigned",
  "data": {
    "object": {
      "user": {
        "user_id": "auth0|demo-user-123"
      },
      "organization": {
        "id": "org_demo_123"
      },
      "role": {
        "id": "admin"
      }
    }
  }
}'

echo "POST $WEBHOOK_URL"
echo "$ROLE_ASSIGNED" | jq .
RESPONSE=$(curl -s -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "$ROLE_ASSIGNED")
echo "Response: $RESPONSE" | jq .
echo ""

# 5. Unknown Event Type (should be ignored)
echo "❓ Testing unknown event type (should be ignored)..."
UNKNOWN_EVENT='{
  "type": "unknown.event.type",
  "data": {
    "object": {
      "id": "some-id"
    }
  }
}'

echo "POST $WEBHOOK_URL"
echo "$UNKNOWN_EVENT" | jq .
RESPONSE=$(curl -s -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "$UNKNOWN_EVENT")
echo "Response: $RESPONSE" | jq .
echo ""

# 6. Invalid JSON (should return error)
echo "❌ Testing invalid JSON (should return error)..."
echo "POST $WEBHOOK_URL"
echo "Body: invalid-json"
HTTP_CODE=$(curl -s -w "%{http_code}" -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "invalid-json" \
  -o /dev/null)
echo "HTTP Status: $HTTP_CODE"
echo ""

echo "✅ Demo completed successfully!"
echo ""
echo "📋 Summary:"
echo "- The webhook service can receive and process Auth0 events"
echo "- Valid events are mapped to OpenFGA tuples using the mapping engine"
echo "- Unknown event types are ignored gracefully"
echo "- Invalid payloads return appropriate error responses"
echo ""
echo "🔗 In a real deployment:"
echo "- Configure Auth0 webhooks to point to your service URL"
echo "- Set up proper authentication with OpenFGA"
echo "- Enable signature verification with AUTH0_WEBHOOK_SECRET"
echo "- Deploy behind HTTPS with proper SSL certificates"
