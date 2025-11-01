# EVE Library Fixes - RESOLVED ✅

**Date Created:** 2025-10-27
**Date Resolved:** 2025-10-27
**Project:** Graphium
**EVE Version:** v0.0.6+ (local /home/opunix/eve)
**Status:** ✅ **FIXED** - All issues resolved

---

## Resolution Summary

The EVE library's `SaveGenericDocument()` function has been fixed and verified working:

✅ **Fix Verified:** Host documents now properly persist to CouchDB
✅ **Test Passed:** Created test-host-002 successfully saved with _id and _rev
✅ **Agent Working:** Docker agent successfully registered localhost-docker host
✅ **Graph Complete:** 7 hosts, 109 containers, all edges valid

**Graphium Changes Required:**
- Fixed type mismatch in ListHosts() - accept both ComputerServer and ComputerSystem
- Commit: 21fd570 - "fix(storage): Accept both ComputerServer and ComputerSystem types"
- Commit: ac70a6a - "fix(graph): Skip edges for non-existent hosts"

**Testing Results:**
```bash
# Host creation now works
curl -X POST http://localhost:8095/api/v1/hosts -d @test-host.json
# Returns: HTTP 201 Created ✅

# Document exists in CouchDB
curl -u admin:testpass http://localhost:5985/graphium/test-host-002
# Returns: {"_id":"test-host-002","_rev":"1-xxx",...} ✅

# Agent registers successfully
./graphium agent --config configs/config.yaml
# Logs: "✓ Host registered: fedora (localhost-docker)" ✅
```

---

## Original Issue Report

~~**Issue:** SaveGenericDocument silently fails - no documents are persisted~~
**Status:** FIXED ✅

---

## Critical Issue: SaveGenericDocument Not Persisting Documents

### Problem Description

`SaveGenericDocument()` returns `nil` (no error) but documents are **NOT being saved to CouchDB**.

### Evidence

1. **API Response:**
   - POST /api/v1/hosts returns HTTP 201 Created ✅
   - Response body contains the host object ✅

2. **Database Reality:**
   - CouchDB shows document does NOT exist ❌
   - Query: `curl -u admin:testpass http://localhost:5985/graphium/test-host-001`
   - Result: `{"error":"not_found","reason":"missing"}`

3. **Test Case:**
   ```bash
   # Create host via API
   curl -X POST http://localhost:8095/api/v1/hosts \
     -H "Content-Type: application/json" \
     -d '{
       "@context": "https://schema.org",
       "@type": "ComputerSystem",
       "@id": "test-host-001",
       "name": "test-server",
       "ipAddress": "192.168.1.100",
       "cpu": 4,
       "memory": 8589934592,
       "status": "active",
       "location": "test-dc"
     }'

   # Result: HTTP 201 Created (but document not in DB)

   # Verify in CouchDB
   curl -u admin:testpass http://localhost:5985/graphium/test-host-001
   # Result: {"error":"not_found"}
   ```

4. **Code Path:**
   ```
   API Handler (handlers_hosts.go:80)
     → s.storage.SaveHost(&host)
       → storage/storage.go:275
         → s.service.SaveGenericDocument(host)
           → EVE Library (FAILING SILENTLY)
   ```

---

## Technical Details

### Function Signature

The failing function in EVE library:
```go
func (s *Service) SaveGenericDocument(doc interface{}) (string, error)
```

**Expected behavior:**
- Save document to CouchDB
- Return document ID and error (if any)

**Actual behavior:**
- Returns `nil` error (success)
- Document is NOT saved to database

### Graphium's Usage

**File:** `internal/storage/storage.go:266-277`

```go
func (s *Storage) SaveHost(host *models.Host) error {
	// Set JSON-LD context and type if not set
	if host.Context == "" {
		host.Context = "https://schema.org"
	}
	if host.Type == "" {
		host.Type = "ComputerServer"
	}

	_, err := s.service.SaveGenericDocument(host)
	return err  // Returns nil (no error) but doc not saved!
}
```

### Document Structure

**Model:** `models/host.go`

```go
type Host struct {
	Context    string `json:"@context" jsonld:"@context"`
	Type       string `json:"@type" jsonld:"@type"`
	ID         string `json:"@id" jsonld:"@id" couchdb:"_id"`
	Rev        string `json:"_rev,omitempty" couchdb:"_rev"`
	Name       string `json:"name" jsonld:"name" couchdb:"required,index"`
	IPAddress  string `json:"ipAddress" jsonld:"ipAddress" couchdb:"required,index"`
	CPU        int    `json:"cpu" jsonld:"processorCount"`
	Memory     int64  `json:"memory" jsonld:"memorySize"`
	Status     string `json:"status" jsonld:"status" couchdb:"index"`
	Datacenter string `json:"location" jsonld:"location" couchdb:"index"`
}
```

**Sample Document:**
```json
{
  "@context": "https://schema.org",
  "@type": "ComputerSystem",
  "@id": "test-host-001",
  "name": "test-server",
  "ipAddress": "192.168.1.100",
  "cpu": 4,
  "memory": 8589934592,
  "status": "active",
  "location": "test-dc"
}
```

---

## Impact

### Immediate Impact

1. **Host Registration Fails Silently**
   - Docker agent registers host with API ✅
   - Agent logs "✓ Host registered: fedora (localhost-docker)" ✅
   - Host NOT in database ❌

2. **Containers Become Orphaned**
   - Agent syncs 109 containers with `hostedOn: "localhost-docker"`
   - Containers saved successfully ✅
   - Host "localhost-docker" doesn't exist ❌
   - Graph visualization fails ❌

3. **Graph Visualization Breaks**
   - Cytoscape.js error: "Can not create edge with nonexistant target localhost-docker"
   - Graph fails to render
   - User sees error in browser console

### Cascading Effects

- Any new host registration fails silently
- Multi-host deployments impossible
- Graph topology incomplete
- Real-time monitoring broken for new hosts

---

## Required Fixes in EVE Library

### Fix #1: Ensure SaveGenericDocument Actually Saves

**Location:** EVE library's `SaveGenericDocument()` function

**Requirements:**
1. Document must be persisted to CouchDB
2. Return proper error if save fails
3. Return document ID on success
4. Handle document ID from `@id` field correctly
5. Handle `_id` and `_rev` fields properly

**Test Case:**
```go
// This should work:
host := &models.Host{
    Context:    "https://schema.org",
    Type:       "ComputerSystem",
    ID:         "test-host-001",  // Should map to _id
    Name:       "test-server",
    IPAddress:  "192.168.1.100",
    CPU:        4,
    Memory:     8589934592,
    Status:     "active",
    Datacenter: "test-dc",
}

docID, err := service.SaveGenericDocument(host)
// Expected: docID = "test-host-001", err = nil, document EXISTS in DB
// Actual: docID = something, err = nil, document MISSING in DB
```

### Fix #2: Proper ID Field Mapping

**Issue:** The EVE library might not be properly mapping `@id` to CouchDB's `_id`.

**Requirements:**
1. Recognize `@id` field from JSON-LD
2. Map it to CouchDB's `_id`
3. Preserve `_rev` for updates
4. Handle both `@id` and `_id` in the struct tags

**Current Tags:**
```go
ID string `json:"@id" jsonld:"@id" couchdb:"_id"`
```

### Fix #3: Error Handling

**Issue:** Function returns `nil` error even when save fails.

**Requirements:**
1. Return error if CouchDB returns error response
2. Return error if document not found after save
3. Return error if ID generation fails
4. Don't silently swallow errors

---

## Debugging Information

### CouchDB Configuration

```yaml
couchdb:
  url: http://localhost:5985
  database: graphium
  username: admin
  password: testpass
```

### CouchDB Status

```bash
$ curl -u admin:testpass http://localhost:5985/graphium
{
  "db_name": "graphium",
  "doc_count": 5803,  # Containers + test data
  "doc_del_count": 0,
  ...
}
```

### Documents That DO Work

Containers save successfully via `SaveContainer()`:
```bash
$ curl -u admin:testpass http://localhost:5985/graphium/_all_docs | jq '.total_rows'
5803  # All containers
```

### Documents That DON'T Work

Hosts via `SaveHost()`:
```bash
$ curl -u admin:testpass 'http://localhost:5985/graphium/_all_docs' | \
  jq '.rows | map(.id) | map(select(startswith("host-") or startswith("localhost-")))'
[]  # Empty - no hosts except test data
```

---

## Suggested Fixes (Implementation)

### Option 1: Fix SaveGenericDocument

Add proper CouchDB save logic:
```go
func (s *Service) SaveGenericDocument(doc interface{}) (string, error) {
    // 1. Extract _id from doc (check @id, _id fields)
    // 2. Marshal to JSON
    // 3. PUT/POST to CouchDB with proper auth
    // 4. Check response status
    // 5. Parse response for _rev
    // 6. Update doc with _rev
    // 7. Return _id and error

    // Don't just return (docID, nil) without actually saving!
}
```

### Option 2: Add Validation

Before returning success, verify document exists:
```go
func (s *Service) SaveGenericDocument(doc interface{}) (string, error) {
    // ... save logic ...

    // Verify save succeeded
    _, err := s.GetGenericDocument(docID, doc)
    if err != nil {
        return "", fmt.Errorf("save failed - document not found after save: %w", err)
    }

    return docID, nil
}
```

### Option 3: Add Logging

At minimum, add logging to understand what's happening:
```go
func (s *Service) SaveGenericDocument(doc interface{}) (string, error) {
    log.Printf("SaveGenericDocument: attempting to save doc type=%T", doc)

    // ... save logic ...

    if err != nil {
        log.Printf("SaveGenericDocument: ERROR - %v", err)
        return "", err
    }

    log.Printf("SaveGenericDocument: SUCCESS - saved doc ID=%s", docID)
    return docID, nil
}
```

---

## Testing Requirements

### Unit Test

```go
func TestSaveGenericDocument_Host(t *testing.T) {
    service := setupTestService(t)

    host := &models.Host{
        Context:    "https://schema.org",
        Type:       "ComputerSystem",
        ID:         "test-host-123",
        Name:       "test-server",
        IPAddress:  "10.0.0.1",
        CPU:        8,
        Memory:     16000000000,
        Status:     "active",
        Datacenter: "dc1",
    }

    // Save document
    docID, err := service.SaveGenericDocument(host)
    require.NoError(t, err)
    require.Equal(t, "test-host-123", docID)

    // Verify document exists in CouchDB
    var retrieved models.Host
    err = service.GetGenericDocument(docID, &retrieved)
    require.NoError(t, err)
    require.Equal(t, host.Name, retrieved.Name)
    require.Equal(t, host.IPAddress, retrieved.IPAddress)
}
```

### Integration Test

```bash
#!/bin/bash
# Test host creation end-to-end

# 1. Create host via API
curl -X POST http://localhost:8095/api/v1/hosts \
  -H "Content-Type: application/json" \
  -d '{
    "@context": "https://schema.org",
    "@type": "ComputerSystem",
    "@id": "integration-test-host",
    "name": "integration-test",
    "ipAddress": "10.0.0.100",
    "cpu": 4,
    "memory": 8000000000,
    "status": "active",
    "location": "test"
  }'

# 2. Verify via API
curl http://localhost:8095/api/v1/hosts/integration-test-host | jq .

# 3. Verify in CouchDB directly
curl -u admin:testpass http://localhost:5985/graphium/integration-test-host | jq .

# All three should return the host document
```

---

## Priority

**CRITICAL** - Blocks:
- Host registration
- Multi-host deployments
- Graph visualization
- Real-time agent monitoring

---

## Previously Fixed Issues

For reference, these EVE issues were already fixed:

1. ✅ Index creation API format (commit 880352f)
2. ✅ Query selector wrapper (commit 880352f)
3. ✅ Count operations (commit 880352f)

---

## Appendix: Full Test Scenario

### Setup

```bash
# Start Graphium
./graphium server --config configs/config.yaml &

# Start agent
./graphium agent --config configs/config.yaml &
```

### Observe Bug

```bash
# Agent logs show:
# "✓ Host registered: fedora (localhost-docker)"

# But database check shows:
curl -u admin:testpass http://localhost:5985/graphium/localhost-docker
# {"error":"not_found","reason":"missing"}

# API returns empty (except test data):
curl http://localhost:8095/api/v1/hosts | jq '.hosts | map(."@id")'
# ["host-001"]  ← Only test data

# Graph fails:
# Browser console: "Error: Can not create edge with nonexistant target localhost-docker"
```

### Expected Behavior

```bash
# After fix, database should have:
curl -u admin:testpass http://localhost:5985/graphium/localhost-docker
{
  "@context": "https://schema.org",
  "@type": "ComputerSystem",
  "_id": "localhost-docker",
  "_rev": "1-xxx",
  "name": "fedora",
  "ipAddress": "host-fedora",
  "cpu": 8,
  "memory": 17179869184,
  "status": "active",
  "location": "local"
}

# API should return:
curl http://localhost:8095/api/v1/hosts | jq '.hosts | map(."@id")'
# ["host-001", "localhost-docker"]

# Graph should render without errors
```

---

**End of Document**
