# Task System Testing Instructions

## Quick Test (Option 1)

This test verifies the agent task system works end-to-end without requiring full stack handler integration.

### Prerequisites

1. **Server running** on port 8095
2. **Agent token** (JWT) for authentication
3. **Agent running** (optional - to see task execution)

### Step 1: Build the Project

```bash
cd /home/opunix/graphium
go build -o graphium-dev ./cmd/graphium
```

### Step 2: Start the Server

```bash
./graphium-dev server
```

Leave this running in one terminal.

### Step 3: Run the Test Script

In another terminal:

```bash
# Set your agent token (if you have auth enabled)
export TOKEN="your-jwt-token-here"

# Run the test
./test-task-system.sh
```

### Expected Output

```
===================================
Agent Task System Test
===================================

1. Checking if server is running... ✓
2. Creating test deployment task... ✓
   Task ID: task-test-1234567890
3. Checking if agent can see the task... ✓
   Task is visible to agent
4. Checking task statistics... ✓
   Total tasks: 1, Pending: 1
5. Retrieving task details... ✓
   Task status: pending

===================================
Test Summary
===================================
✓ Task system is functional
✓ Tasks can be created via API
✓ Tasks are stored in database
✓ Tasks are visible to agents

Next steps:
1. Start an agent to process the task
2. Watch the agent pick up and execute the task
3. Verify the container was created
4. Clean up the test container
```

### Step 4: Start an Agent (Optional)

To see the task actually execute:

```bash
TOKEN="your-jwt-token" ./graphium-dev agent --host-id localhost-docker
```

Watch the agent logs - you should see:
- "Polling for tasks..."
- "Found 1 task(s)"
- "Executing task task-test-..."
- "Successfully deployed container..."

### Step 5: Verify Container Created

```bash
docker ps | grep test-nginx
```

You should see a nginx container running on port 8888.

### Step 6: Test the Container

```bash
curl http://localhost:8888
```

Should return the nginx welcome page.

### Step 7: Cleanup

```bash
docker rm -f $(docker ps -q -f label=test=true)
```

## Troubleshooting

### Server not running
```bash
# Check if port 8095 is in use
lsof -i :8095

# Start server if needed
./graphium-dev server
```

### Authentication errors
If you see "Unauthorized" errors, you need to either:
1. Disable auth in config
2. Generate a valid agent token
3. Use TOKEN="" for unauthenticated requests (if auth is disabled)

### Task not visible to agent
- Wait 1-2 seconds for database writes to propagate
- Check server logs for errors
- Verify agent ID matches (should be "localhost-docker")

## Success Criteria

- ✅ Task created via API
- ✅ Task stored in CouchDB
- ✅ Task visible to agent via polling endpoint
- ✅ Task statistics endpoint returns correct counts
- ✅ Task can be retrieved by ID
- ✅ Agent can execute task (if agent running)
- ✅ Container deployed successfully (if agent running)
