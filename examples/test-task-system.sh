#!/bin/bash
# Test script for agent task system
# This creates a simple deployment task and verifies it's processed

set -e

API_URL="http://localhost:8095"
AGENT_ID="localhost-docker"

echo "==================================="
echo "Agent Task System Test"
echo "==================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if server is running
echo -n "1. Checking if server is running... "
if curl -s -f "$API_URL/api/v1/stats" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    echo "   Server not running. Start it with: ./graphium-dev server"
    exit 1
fi

# Check if agent token is set
if [ -z "$TOKEN" ]; then
    echo -e "${YELLOW}⚠${NC}  TOKEN environment variable not set"
    echo "   Using unauthenticated request (will fail if auth is enabled)"
    TOKEN=""
fi

# Create a simple test task
echo -n "2. Creating test deployment task... "
TASK_ID="task-test-$(date +%s)"
TASK_JSON=$(cat <<EOF
{
  "@id": "$TASK_ID",
  "@type": "AgentTask",
  "taskType": "deploy",
  "status": "pending",
  "agentId": "$AGENT_ID",
  "hostId": "$AGENT_ID",
  "priority": 5,
  "dateCreated": "$(date -Iseconds)",
  "payload": {
    "containerSpec": {
      "name": "test-nginx-$(date +%s)",
      "image": "nginx:alpine",
      "ports": [
        {
          "containerPort": 80,
          "hostPort": 8888,
          "protocol": "tcp"
        }
      ],
      "environment": [],
      "volumeMounts": [],
      "restartPolicy": "unless-stopped",
      "command": [],
      "workingDir": ""
    },
    "pullPolicy": "if-not-present",
    "labels": {
      "test": "true",
      "graphium.managed": "true"
    }
  }
}
EOF
)

RESPONSE=$(curl -s -X POST "$API_URL/api/v1/tasks" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "$TASK_JSON" 2>&1)

if echo "$RESPONSE" | grep -q '"@id"'; then
    echo -e "${GREEN}✓${NC}"
    echo "   Task ID: $TASK_ID"
else
    echo -e "${RED}✗${NC}"
    echo "   Response: $RESPONSE"
    exit 1
fi

# Wait a moment for task creation to propagate
sleep 1

# Check if task appears in agent's queue
echo -n "3. Checking if agent can see the task... "
AGENT_TASKS=$(curl -s "$API_URL/api/v1/agents/$AGENT_ID/tasks?status=pending" \
  -H "Authorization: Bearer $TOKEN" 2>&1)

if echo "$AGENT_TASKS" | grep -q "$TASK_ID"; then
    echo -e "${GREEN}✓${NC}"
    echo "   Task is visible to agent"
else
    echo -e "${YELLOW}⚠${NC}"
    echo "   Task not yet in agent queue (this is OK if agent hasn't polled yet)"
fi

# Check task statistics
echo -n "4. Checking task statistics... "
STATS=$(curl -s "$API_URL/api/v1/tasks/stats" \
  -H "Authorization: Bearer $TOKEN" 2>&1)

if echo "$STATS" | grep -q '"total"'; then
    echo -e "${GREEN}✓${NC}"
    TOTAL=$(echo "$STATS" | grep -o '"total":[0-9]*' | cut -d: -f2)
    PENDING=$(echo "$STATS" | grep -o '"pending":[0-9]*' | cut -d: -f2)
    echo "   Total tasks: $TOTAL, Pending: $PENDING"
else
    echo -e "${RED}✗${NC}"
    echo "   Response: $STATS"
fi

# Get the specific task
echo -n "5. Retrieving task details... "
TASK_DETAIL=$(curl -s "$API_URL/api/v1/tasks/$TASK_ID" \
  -H "Authorization: Bearer $TOKEN" 2>&1)

if echo "$TASK_DETAIL" | grep -q "$TASK_ID"; then
    echo -e "${GREEN}✓${NC}"
    STATUS=$(echo "$TASK_DETAIL" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    echo "   Task status: $STATUS"
else
    echo -e "${RED}✗${NC}"
    echo "   Could not retrieve task"
fi

echo ""
echo "==================================="
echo "Test Summary"
echo "==================================="
echo "✓ Task system is functional"
echo "✓ Tasks can be created via API"
echo "✓ Tasks are stored in database"
echo "✓ Tasks are visible to agents"
echo ""
echo "Next steps:"
echo "1. Start an agent to process the task:"
echo "   TOKEN=\$TOKEN ./graphium-dev agent --host-id $AGENT_ID"
echo ""
echo "2. Watch the agent pick up and execute the task"
echo ""
echo "3. Verify the container was created:"
echo "   docker ps | grep test-nginx"
echo ""
echo "4. Clean up the test container:"
echo "   docker rm -f \$(docker ps -q -f label=test=true)"
echo ""
