#!/bin/bash
# Test task system by creating tasks directly via API

set -e

API_URL="http://localhost:8095"

echo "==========================================="
echo "Direct Task Creation Test"
echo "==========================================="
echo ""

# Create a deployment task for localhost-docker
echo "1. Creating deployment task for localhost-docker..."
TASK_1=$(cat <<'EOF'
{
  "@id": "task-test-nginx-localhost",
  "@type": "AgentTask",
  "taskType": "deploy",
  "status": "pending",
  "agentId": "localhost-docker",
  "hostId": "localhost-docker",
  "priority": 5,
  "dateCreated": "2025-10-31T13:00:00Z",
  "payload": {
    "containerSpec": {
      "name": "test-task-nginx-1",
      "image": "nginx:alpine",
      "ports": [
        {
          "containerPort": 80,
          "hostPort": 9091,
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

# Note: We need to use a file because of curl limitations with heredoc
echo "$TASK_1" > /tmp/task1.json
curl -s -X POST "${API_URL}/api/v1/tasks" \
  -H "Content-Type: application/json" \
  -d @/tmp/task1.json | python3 -m json.tool 2>/dev/null || curl -s -X POST "${API_URL}/api/v1/tasks" \
  -H "Content-Type: application/json" \
  -d @/tmp/task1.json

echo ""
echo ""
echo "2. Waiting 3 seconds for task to be processed..."
sleep 3

echo ""
echo "3. Checking task statistics:"
curl -s "${API_URL}/api/v1/tasks/stats" | python3 -m json.tool 2>/dev/null

echo ""
echo ""
echo "4. Listing all tasks:"
curl -s "${API_URL}/api/v1/tasks" | python3 -m json.tool 2>/dev/null | head -80

echo ""
echo ""
echo "5. Checking localhost-docker containers:"
docker ps --filter "label=test=true" --format "table {{.Names}}\t{{.Image}}\t{{.Ports}}\t{{.Status}}"

echo ""
echo "========================================="
echo "Test Complete"
echo "========================================="
echo ""
echo "If you see a container above, the task system is working!"
echo ""

# Cleanup
rm -f /tmp/task1.json
