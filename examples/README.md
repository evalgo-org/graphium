# Graphium Scheduled Actions Examples

This directory contains example JSON-LD files demonstrating how to use Graphium's scheduled actions feature.

## Health Check Action Example

The `health-check-action.jsonld` file demonstrates a simple periodic health check that monitors a web service endpoint.

### Example Structure

```json
{
  "@context": "https://schema.org",
  "@type": "CheckAction",
  "name": "Web Service Health Check",
  "description": "Periodic health check for the web service on port 8080...",
  "agent": "localhost-docker",
  "actionStatus": "PotentialActionStatus",
  "enabled": true,
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "PT5M",
    "scheduleTimezone": "UTC"
  },
  "instrument": {
    "url": "http://localhost:8080/health",
    "method": "GET",
    "expectedStatusCode": 200,
    "timeout": 5
  }
}
```

### Field Descriptions

#### Top-Level Fields

- **@context**: Always set to `"https://schema.org"` for schema.org compliance
- **@type**: Action type - use `"CheckAction"` for health checks
- **name**: Human-readable name for the scheduled action
- **description**: Detailed description of what this action does
- **agent**: The agent (host) ID that will execute this action (e.g., "localhost-docker", "vm1")
- **actionStatus**: Initial status, typically `"PotentialActionStatus"` for new actions
- **enabled**: Boolean flag to enable/disable the action

#### Schedule Fields

- **@type**: Always `"Schedule"` for schema.org compliance
- **repeatFrequency**: ISO 8601 duration format specifying how often to run:
  - `PT5M` = Every 5 minutes
  - `PT1H` = Every 1 hour
  - `PT30S` = Every 30 seconds
  - `P1D` = Every 1 day
  - `P1W` = Every 1 week
- **scheduleTimezone**: Timezone for schedule evaluation (e.g., "UTC", "America/New_York")

#### Instrument Fields (Health Check Parameters)

- **url**: The HTTP endpoint to check
- **method**: HTTP method to use (GET, POST, etc.)
- **expectedStatusCode**: HTTP status code expected for a successful check (typically 200)
- **timeout**: Request timeout in seconds

### Usage

#### Via Web UI

1. Navigate to the Actions page in Graphium's web interface
2. Click "Create Action"
3. Fill in the form with values from the example
4. Submit to create the scheduled action

#### Via REST API

Create the action using a POST request:

```bash
curl -X POST http://localhost:8095/api/v1/actions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d @health-check-action.jsonld
```

#### View Action Status

After creation, you can:
- View the action in the web UI at `/web/actions`
- Check execution history to see health check results
- Monitor task creation and completion

### How It Works

1. **Scheduler Evaluation**: The Graphium scheduler evaluates all enabled actions every 30 seconds
2. **Task Creation**: When it's time to execute, the scheduler creates an AgentTask with type "check"
3. **Agent Polling**: The agent polls for pending tasks every 5 seconds
4. **Execution**: The agent executes the health check by making an HTTP request to the specified URL
5. **Result Recording**: The task result (success/failure, response time, status code) is sent back to the server
6. **History**: All executions are recorded and viewable in the action's history

### Advanced Examples

#### Health Check with Custom Headers

```json
{
  "@context": "https://schema.org",
  "@type": "CheckAction",
  "name": "API Health Check with Auth",
  "agent": "localhost-docker",
  "enabled": true,
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "PT10M",
    "scheduleTimezone": "UTC"
  },
  "instrument": {
    "url": "http://api.example.com/health",
    "method": "GET",
    "expectedStatusCode": 200,
    "timeout": 10,
    "headers": {
      "Authorization": "Bearer token123",
      "X-Custom-Header": "value"
    }
  }
}
```

#### Daily Health Check at Specific Time

```json
{
  "@context": "https://schema.org",
  "@type": "CheckAction",
  "name": "Daily Morning Health Check",
  "agent": "localhost-docker",
  "enabled": true,
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "P1D",
    "scheduleTimezone": "America/New_York",
    "startDate": "2025-10-31T09:00:00-04:00"
  },
  "instrument": {
    "url": "http://localhost:8080/health",
    "method": "GET",
    "expectedStatusCode": 200,
    "timeout": 5
  }
}
```

#### Health Check on Specific Days

```json
{
  "@context": "https://schema.org",
  "@type": "CheckAction",
  "name": "Weekday Business Hours Check",
  "agent": "localhost-docker",
  "enabled": true,
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "PT1H",
    "scheduleTimezone": "UTC",
    "byDay": ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
  },
  "instrument": {
    "url": "http://localhost:8080/api/status",
    "method": "GET",
    "expectedStatusCode": 200,
    "timeout": 5
  }
}
```

## Supported Action Types

Currently, Graphium supports the following schema.org action types:

- **CheckAction**: Health checks and endpoint monitoring
- **ControlAction**: Container control operations (start, stop, restart)
- **CreateAction**: Container deployment and creation
- **UpdateAction**: Container updates and modifications
- **TransferAction**: Container migration between hosts

## Repeat Frequency Formats

Graphium supports ISO 8601 duration format for `repeatFrequency`:

### Time-based Durations (PT prefix)
- `PT30S` - Every 30 seconds
- `PT1M` - Every 1 minute
- `PT5M` - Every 5 minutes
- `PT1H` - Every 1 hour
- `PT2H` - Every 2 hours

### Date-based Durations (P prefix)
- `P1D` - Every 1 day
- `P7D` - Every 7 days
- `P1W` - Every 1 week
- `P1M` - Every 1 month (approximate, 30 days)

## See Also

- [Scheduled Actions API Documentation](../docs/swagger.yaml)
- [Schema.org Action Types](https://schema.org/Action)
- [ISO 8601 Duration Format](https://en.wikipedia.org/wiki/ISO_8601#Durations)
