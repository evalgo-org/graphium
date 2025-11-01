#!/bin/bash
# Deploy nginx-multihost-stack.json via web interface

set -e

API_URL="http://localhost:8095"

echo "========================================="
echo "Deploying nginx-multihost-stack"
echo "========================================="
echo ""

# Read the stack JSON
STACK_JSON=$(cat nginx-multihost-stack.json)

# Deploy via web form
echo "Deploying stack..."
RESPONSE=$(curl -s -X POST "${API_URL}/web/stacks/deploy" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "stack_json=${STACK_JSON}")

# Check if redirect happened (successful deployment)
if echo "$RESPONSE" | grep -q "See Other\|Found\|Moved"; then
    echo "✓ Stack deployment initiated successfully"
    echo ""
    echo "Check deployment status:"
    echo "  - Web UI: http://localhost:8095/web/stacks"
    echo "  - Server logs for task creation"
    echo "  - Agent logs for task execution"
else
    echo "Response:"
    echo "$RESPONSE" | head -50
fi

echo ""
echo "Waiting 5 seconds for tasks to be created..."
sleep 5

# Check task statistics
echo ""
echo "Task statistics:"
curl -s "${API_URL}/api/v1/tasks/stats" | python3 -m json.tool 2>/dev/null || curl -s "${API_URL}/api/v1/tasks/stats"

echo ""
echo ""
echo "List all tasks:"
curl -s "${API_URL}/api/v1/tasks" | python3 -m json.tool 2>/dev/null | head -100 || curl -s "${API_URL}/api/v1/tasks" | head -100

echo ""
