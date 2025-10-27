# Integration Tests

This directory contains integration tests for the Graphium API that test the full stack including the database.

## Prerequisites

Before running integration tests, ensure you have:

1. **CouchDB running** on localhost:5985 (or update the test config)
2. **Database credentials** configured (default: admin/testpass)
3. **Test database** will be created/used: `graphium_test`

## Running Integration Tests

### Run all integration tests:
```bash
go test -v -tags=integration ./tests/integration/...
```

### Run a specific test:
```bash
go test -v -tags=integration ./tests/integration/... -run TestContainerCRUD
```

### Skip integration tests (default):
```bash
go test ./...  # Integration tests are skipped by default
```

## Test Coverage

The integration test suite covers:

### Container CRUD Operations
- **CREATE**: POST /api/v1/containers
- **READ**: GET /api/v1/containers/:id
- **UPDATE**: PUT /api/v1/containers/:id
- **DELETE**: DELETE /api/v1/containers/:id
- **LIST**: GET /api/v1/containers with pagination

### Host CRUD Operations
- **CREATE**: POST /api/v1/hosts
- **READ**: GET /api/v1/hosts/:id
- **UPDATE**: PUT /api/v1/hosts/:id
- **DELETE**: DELETE /api/v1/hosts/:id

### API Validation
- Content-Type validation
- Missing required fields
- Invalid ID formats
- Invalid query parameters

### Bulk Operations
- Bulk container creation
- Bulk result reporting

### Health Checks
- Health endpoint verification
- Database connectivity

## Test Data

Integration tests use test-specific IDs to avoid conflicts:
- Containers: `test-container-*`, `bulk-container-*`
- Hosts: `test-host-*`

Tests clean up after themselves by deleting created resources.

## Configuration

Test configuration is defined in `getTestConfig()`:
- Server: localhost:8095
- CouchDB: localhost:5985
- Database: graphium_test
- Credentials: admin/testpass

To use different settings, modify the `getTestConfig()` function or set environment variables.

## Troubleshooting

### Database Connection Errors
```
Failed to create storage: connection refused
```
**Solution**: Ensure CouchDB is running on localhost:5985

### Authentication Errors
```
unauthorized: admin credentials invalid
```
**Solution**: Verify CouchDB credentials match the test config

### Port Already in Use
```
bind: address already in use
```
**Solution**: The tests use httptest which doesn't bind to actual ports, so this shouldn't occur

## CI/CD Integration

To run integration tests in CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Start CouchDB
  run: docker run -d -p 5985:5984 -e COUCHDB_USER=admin -e COUCHDB_PASSWORD=testpass couchdb:3.3

- name: Run Integration Tests
  run: go test -v -tags=integration ./tests/integration/...
```

## Best Practices

1. **Always use test-specific IDs** to avoid conflicts with production data
2. **Clean up resources** after each test
3. **Use subtests** (`t.Run()`) to organize related tests
4. **Test both success and failure paths**
5. **Verify HTTP status codes** and response bodies
6. **Test middleware and validation** as part of integration tests
