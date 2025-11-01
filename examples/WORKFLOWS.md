# Graphium Workflows

Workflows allow you to compose complex deployment scenarios from multiple actions that execute sequentially, with the ability to pass data between steps using variable substitution.

## Overview

Workflows are implemented as **composite actions** that contain multiple sub-actions. Each action can:
- Execute commands inside containers (`container-exec`)
- Deploy container stacks (`deploy-stack`)
- Wait for health conditions (`wait`)
- Reference outputs from previous actions using `${{variable.path}}`

## Workflow Structure

A workflow is defined using the Schema.org `instrument` field with these properties:

```json
{
  "compositeAction": true,
  "executionMode": "sequential",
  "actions": [
    {
      "type": "action-type",
      "outputAs": "variableName",
      "description": "What this action does",
      ... action-specific fields
    }
  ]
}
```

### Properties

- **compositeAction** (boolean, required): Set to `true` to indicate this is a workflow
- **executionMode** (string): Execution mode - currently only `"sequential"` is supported
- **actions** (array, required): Array of actions to execute in order

### Action Types

#### 1. container-exec

Execute a command inside a running container.

**Fields:**
- `type`: `"container-exec"`
- `containerId`: Container ID or name
- `command`: Command to execute (string, array, or shell command)
- `workDir` (optional): Working directory inside container
- `env` (optional): Environment variables as object
- `user` (optional): User to run command as
- `outputAs` (optional): Variable name to store output

**Command formats:**
```json
// Single string - wrapped in /bin/sh -c
"command": "echo hello"

// Array - executed directly
"command": ["ls", "-la", "/app"]

// Shell command with pipe
"command": "cat /app/config.json | grep version"
```

**Output data:**
- `container_id`: Container that executed the command
- `command`: Command that was executed
- `exit_code`: Exit code of the command
- `output`: Combined stdout/stderr output
- `duration_ms`: Execution time in milliseconds
- `exec_id`: Docker exec instance ID

#### 2. deploy-stack

Deploy a container stack (placeholder - to be implemented).

**Fields:**
- `type`: `"deploy-stack"`
- `stackId`: Stack identifier
- `env` (optional): Environment variables for the stack
- `outputAs` (optional): Variable name to store deployment info

#### 3. wait

Wait for a condition or health check (placeholder - to be implemented).

**Fields:**
- `type`: `"wait"`
- `containerId`: Container to wait for
- `healthCheck`: Health check configuration
- `outputAs` (optional): Variable name to store result

## Variable Substitution

Use `${{variableName.field.subfield}}` syntax to reference outputs from previous actions.

### Example

```json
{
  "actions": [
    {
      "type": "deploy-stack",
      "stackId": "postgres",
      "outputAs": "database"
    },
    {
      "type": "container-exec",
      "containerId": "${{database.containers.postgres}}",
      "command": "psql -U admin -c 'CREATE DATABASE myapp;'"
    }
  ]
}
```

The `${{database.containers.postgres}}` reference will be replaced with the actual container ID from the `database` output.

### Substitution Rules

- Variables are replaced recursively in strings, objects, and arrays
- Nested paths are supported: `${{output.nested.field}}`
- If a variable is not found, the original `${{...}}` syntax is preserved
- Variables can only reference outputs from **previous** actions in the sequence

## Complete Examples

### Example 1: Simple Command Execution

File: `examples/simple-exec-workflow.jsonld`

Tests basic container exec with multiple commands:

```json
{
  "@context": "https://schema.org",
  "@type": "UpdateAction",
  "name": "Simple Container Exec Workflow Test",
  "agent": "localhost-docker",
  "instrument": {
    "compositeAction": true,
    "executionMode": "sequential",
    "actions": [
      {
        "type": "container-exec",
        "containerId": "nginx-multihost-nginx-1",
        "command": "echo 'Step 1: Getting container hostname'",
        "outputAs": "step1"
      },
      {
        "type": "container-exec",
        "containerId": "nginx-multihost-nginx-1",
        "command": ["sh", "-c", "hostname && date"],
        "outputAs": "step2"
      },
      {
        "type": "container-exec",
        "containerId": "nginx-multihost-nginx-1",
        "command": ["ls", "-la", "/etc/nginx"],
        "workDir": "/etc",
        "outputAs": "step3"
      }
    ]
  }
}
```

### Example 2: PostgreSQL Database with Migrations

File: `examples/postgres-app-workflow.jsonld`

Complete application deployment with database setup:

```json
{
  "instrument": {
    "compositeAction": true,
    "executionMode": "sequential",
    "actions": [
      {
        "type": "deploy-stack",
        "stackId": "postgres-stack",
        "outputAs": "database",
        "description": "Deploy PostgreSQL database"
      },
      {
        "type": "wait",
        "containerId": "${{database.containers.postgres}}",
        "healthCheck": {
          "command": ["pg_isready", "-U", "admin"],
          "interval": "2s",
          "timeout": "30s"
        },
        "outputAs": "dbReady"
      },
      {
        "type": "container-exec",
        "containerId": "${{database.containers.postgres}}",
        "command": ["psql", "-U", "admin", "-c", "CREATE DATABASE myapp;"],
        "outputAs": "dbCreated"
      },
      {
        "type": "container-exec",
        "containerId": "${{database.containers.postgres}}",
        "command": ["psql", "-U", "admin", "-d", "myapp", "-f", "/migrations/001_initial_schema.sql"],
        "outputAs": "migration1"
      },
      {
        "type": "container-exec",
        "containerId": "${{database.containers.postgres}}",
        "command": "psql -U admin -d myapp -c \"CREATE USER appuser WITH PASSWORD 'secure_password_here';\"",
        "outputAs": "userCreated"
      },
      {
        "type": "deploy-stack",
        "stackId": "myapp-stack",
        "env": {
          "DATABASE_URL": "postgres://appuser:secure_password_here@${{database.containers.postgres}}:5432/myapp"
        },
        "outputAs": "application"
      }
    ]
  }
}
```

## Execution Model

### Sequential Execution

Actions execute in order, one at a time:

1. First action executes
2. If successful and has `outputAs`, output is stored
3. Variables in next action are substituted
4. Next action executes
5. Repeat until all actions complete or one fails

### Error Handling

- If any action fails (exit code != 0 or error), the workflow stops immediately
- The workflow result includes:
  - `failed_step`: Index of the failed action
  - `step_result`: Full result from the failed action
  - Original error details

### Success Result

When all actions complete successfully, the workflow returns:

```json
{
  "success": true,
  "message": "Successfully executed N actions",
  "data": {
    "actions_count": 5,
    "last_result": { /* result from final action */ },
    "action_outputs": {
      "variableName1": { /* output data */ },
      "variableName2": { /* output data */ }
    }
  }
}
```

## Task Type

Workflows use the task type `"workflow"` in the agent task system.

To create a workflow task:

```json
{
  "taskType": "workflow",
  "payload": {
    "compositeAction": true,
    "executionMode": "sequential",
    "actions": [ /* ... */ ]
  }
}
```

## Best Practices

### 1. Use Descriptive Output Names

```json
{
  "outputAs": "postgresDeployed",
  "description": "Deploy PostgreSQL database"
}
```

### 2. Add Descriptions to Each Action

Helps with debugging and understanding the workflow:

```json
{
  "type": "container-exec",
  "description": "Run migration 001: initial schema",
  // ...
}
```

### 3. Check Exit Codes

The workflow automatically stops on non-zero exit codes. Design your commands to return proper exit codes:

```bash
# Good - exits with error if database doesn't exist
psql -U admin -d myapp -c "SELECT 1" || exit 1

# Bad - always exits with 0 even if query fails
psql -U admin -d myapp -c "SELECT 1" ; true
```

### 4. Test Each Action Independently First

Before building a complex workflow, test each action individually to ensure it works correctly.

### 5. Use Explicit Container IDs for Testing

Start with explicit container IDs, then refactor to use variable substitution:

```json
// Testing
{ "containerId": "postgres-test-1", "command": "..." }

// Production
{ "containerId": "${{database.containers.postgres}}", "command": "..." }
```

## Implementation Details

### Files

- `agent/workflow_executor.go` - Workflow orchestration and variable substitution
- `agent/container_exec.go` - Container exec implementation
- `agent/task_executor_workflow.go` - Integration with task executor
- `agent/task_executor.go` - Task routing (workflow case)

### Variable Substitution Engine

Uses regular expressions to match `${{...}}` patterns:

```go
re := regexp.MustCompile(`\$\{\{([^}]+)\}\}`)
```

Supports:
- Nested object paths: `${{db.containers.postgres}}`
- Arrays and maps
- Recursive substitution in all data structures
- Safe fallback if variable not found

## Future Enhancements

- **Parallel execution mode**: Execute multiple actions concurrently
- **Conditional actions**: Skip actions based on conditions
- **Retry logic**: Retry failed actions with backoff
- **Timeouts**: Per-action timeout configuration
- **Health checks**: Robust wait/health-check implementation
- **Rollback**: Automatic rollback on failure
- **Templates**: Reusable workflow templates
- **Variables**: Global workflow variables
