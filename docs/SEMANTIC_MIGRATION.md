# Semantic Migration Guide

This guide explains the migration to full Schema.org semantic types for AgentTask and RequestBody.

## Overview

**Breaking Change**: All legacy fields have been removed. Services must now use canonical Schema.org semantic types.

**Affected Services**:
- graphium (AgentTask model)
- http2amqp (RequestBody model)

## AgentTask Changes

### Removed Legacy Fields

| Legacy Field | Semantic Replacement | Notes |
|-------------|---------------------|-------|
| `TaskType` | `Type` (`@type`) | Use Schema.org Action types (ActivateAction, DeleteAction, etc.) |
| `Status` | `ActionStatus` | Use Schema.org status values (PotentialActionStatus, ActiveActionStatus, etc.) |
| `AgentID` | `HostID` + `Agent` | HostID for filtering, Agent for semantic representation |
| `Payload` | `Object.Properties` | Structured semantic object |
| `Result` | `SemanticResult.Output` | JSON string format |
| `AssignedAt` | N/A | Not needed in semantic model |
| `StartedAt` | `StartTime` | Schema.org property |
| `CompletedAt` | `EndTime` | Schema.org property |
| `ErrorMsg` | `Error.Message` | Structured semantic error |

### Removed Functions

- `Normalize()` - No longer needed
- `TaskTypeToSemanticType()` - Direct type mapping
- `StatusToActionStatus()` - Direct status mapping
- `SemanticTypeToTaskType()` - Reverse conversion removed
- `ActionStatusToStatus()` - Reverse conversion removed

### Status Values

**Old (Legacy)**:
```go
"pending", "assigned", "running", "completed", "failed", "cancelled"
```

**New (Semantic)**:
```go
"PotentialActionStatus"  // pending/assigned
"ActiveActionStatus"     // running
"CompletedActionStatus"  // completed successfully
"FailedActionStatus"     // failed/cancelled
```

### Task Types

**Old (Legacy)**:
```go
"deploy", "delete", "stop", "check", "control", "transfer", "workflow"
```

**New (Semantic - Schema.org Action types)**:
```go
"ActivateAction"    // deploy/start/restart
"DeleteAction"      // delete containers
"DeactivateAction"  // stop containers
"CheckAction"       // health checks
"ControlAction"     // container control operations
"TransferAction"    // log collection, file transfers
"WorkflowAction"    // composite workflows
```

### Migration Example

**Before (Legacy)**:
```json
{
  "id": "task:123",
  "taskType": "deploy",
  "status": "pending",
  "agentId": "agent:vm1",
  "payload": {
    "image": "nginx:latest"
  },
  "startedAt": "2024-01-01T10:00:00Z",
  "completedAt": "2024-01-01T10:05:00Z"
}
```

**After (Semantic)**:
```json
{
  "@context": "https://schema.org",
  "@id": "task:123",
  "@type": "ActivateAction",
  "actionStatus": "PotentialActionStatus",
  "hostId": "agent:vm1",
  "agent": {
    "@type": "SoftwareApplication",
    "name": "agent:vm1"
  },
  "object": {
    "@type": "SoftwareApplication",
    "properties": {
      "image": "nginx:latest"
    }
  },
  "startTime": "2024-01-01T10:00:00Z",
  "endTime": "2024-01-01T10:05:00Z"
}
```

### Code Migration

**Before**:
```go
task := &models.AgentTask{
    TaskType: "deploy",
    Status:   "pending",
    AgentID:  "agent:vm1",
}

// Access payload
var payload DeployPayload
json.Unmarshal(task.Payload, &payload)

// Check status
if task.Status == "completed" {
    // ...
}
```

**After**:
```go
task := &models.AgentTask{
    Type:         "ActivateAction",
    ActionStatus: models.TaskStatusPending,
    HostID:       "agent:vm1",
    Agent: &semantic.SemanticAgent{
        Type: "SoftwareApplication",
        Name: "agent:vm1",
    },
}

// Access payload
var payload DeployPayload
task.GetPayloadAs(&payload)

// Check status
if task.ActionStatus == models.TaskStatusCompleted {
    // ...
}
```

## http2amqp RequestBody Changes

### Removed Legacy Fields

| Legacy Field | Semantic Replacement | Notes |
|-------------|---------------------|-------|
| `InFile` | `Object.ContentUrl` | File reference in semantic object |
| `LegacyID` | `ID` (`@id`) | Use semantic ID format (msg:uuid) |
| `Version` | `Properties["version"]` | Stored in additionalProperty |
| `Process` | `Properties["process"]` | Stored in additionalProperty |

### Removed Functions

- `Normalize()` - No longer needed
- `ToLegacy()` - No backward conversion

### Migration Example

**Before (Legacy)**:
```json
{
  "id": "process-123",
  "version": "v1",
  "in_file": "/data/input.xml",
  "process": "transform"
}
```

**After (Semantic)**:
```json
{
  "@context": "https://schema.org",
  "@id": "msg:process-123",
  "@type": "SendAction",
  "actionStatus": "PotentialActionStatus",
  "object": {
    "@type": "DigitalDocument",
    "contentUrl": "/data/input.xml"
  },
  "additionalProperty": {
    "version": "v1",
    "process": "transform"
  }
}
```

### Code Migration

**Before**:
```go
body := RequestBody{
    LegacyID: "process-123",
    Version:  "v1",
    InFile:   "/data/input.xml",
    Process:  "transform",
}
```

**After**:
```go
body := RequestBody{
    Context:      "https://schema.org",
    Type:         "SendAction",
    ID:           "msg:process-123",
    ActionStatus: "PotentialActionStatus",
    Object: &semantic.SemanticObject{
        Type:       "DigitalDocument",
        ContentUrl: "/data/input.xml",
    },
    Properties: map[string]interface{}{
        "version": "v1",
        "process": "transform",
    },
}
```

## Helper Functions

### AgentTask Helpers

These helper methods work with semantic fields:

```go
// Get payload from Object.Properties
var payload DeployPayload
task.GetPayloadAs(&payload)

// Set payload in Object.Properties
task.SetPayload(payload)

// Get result from SemanticResult.Output
result, err := task.GetResult()

// Set result in SemanticResult.Output
task.SetResult(result)

// Check if task can be retried
if task.CanRetry() {
    // ...
}

// Check if task has expired
if task.IsExpired() {
    // ...
}

// Check if task should execute for agent
if task.ShouldExecute(agentID) {
    // ...
}
```

## API Changes

### Task Creation

**Before**:
```http
POST /api/v1/tasks
{
  "taskType": "deploy",
  "status": "pending",
  "agentId": "agent:vm1",
  "payload": {...}
}
```

**After**:
```http
POST /api/v1/tasks
{
  "@type": "ActivateAction",
  "actionStatus": "PotentialActionStatus",
  "hostId": "agent:vm1",
  "agent": {
    "@type": "SoftwareApplication",
    "name": "agent:vm1"
  },
  "object": {
    "@type": "SoftwareApplication",
    "properties": {...}
  }
}
```

### Task Status Update

**Before**:
```http
PUT /api/v1/tasks/{id}/status
{
  "status": "completed",
  "result": {...}
}
```

**After**:
```http
PUT /api/v1/tasks/{id}/status
{
  "status": "completed",  // Accepts simple values, maps to ActionStatus internally
  "result": {...}
}
```

Note: The API still accepts simple status strings for convenience, but internally maps them to semantic values.

## Database Queries

### Query by Status

**Before**:
```go
tasks := storage.GetTasksByStatus("pending")
```

**After**:
```go
tasks := storage.GetTasksByStatus(models.TaskStatusPending) // "PotentialActionStatus"
```

### Query by Agent

**Before**:
```go
tasks := storage.GetTasksByAgent(agentID, "running")
```

**After**:
```go
tasks := storage.GetTasksByAgent(agentID, models.TaskStatusRunning) // "ActiveActionStatus"
```

## Constants Reference

### Task Status Constants

```go
const (
    TaskStatusPending   = "PotentialActionStatus"
    TaskStatusAssigned  = "PotentialActionStatus"
    TaskStatusRunning   = "ActiveActionStatus"
    TaskStatusCompleted = "CompletedActionStatus"
    TaskStatusFailed    = "FailedActionStatus"
    TaskStatusCancelled = "FailedActionStatus"
)
```

### Action Type Constants

```go
const (
    ActionTypeActivate   = "ActivateAction"
    ActionTypeDeactivate = "DeactivateAction"
    ActionTypeDelete     = "DeleteAction"
    ActionTypeCheck      = "CheckAction"
    ActionTypeControl    = "ControlAction"
    ActionTypeTransfer   = "TransferAction"
    ActionTypeWorkflow   = "WorkflowAction"
)
```

## Benefits

1. **Schema.org Compliance**: Full compatibility with semantic web standards
2. **Workflow Integration**: Direct integration with workflow schedulers (e.g., "when" service)
3. **Type Safety**: Strongly typed semantic structures
4. **Clarity**: Clear semantic meaning for all fields
5. **Extensibility**: Easy to add new semantic properties
6. **Interoperability**: Standard format for cross-service communication

## Breaking Changes Summary

- ❌ All legacy fields removed
- ❌ Normalize() and conversion functions removed
- ✅ Use semantic fields exclusively
- ✅ Use Schema.org Action types and status values
- ✅ Use helper methods for payload/result access

## Support

If you encounter issues migrating:
1. Check this guide for field mappings
2. Review the code examples above
3. Examine the unit tests in the repository
4. Contact the development team

## Related Documentation

- [Schema.org Actions](https://schema.org/Action)
- [EVE Semantic Types](https://github.com/evalgo-org/eve)
- [Workflow Integration Guide](./WORKFLOW_INTEGRATION.md)
