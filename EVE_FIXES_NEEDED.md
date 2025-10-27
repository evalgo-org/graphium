# EVE Library - Fixes Needed for Graphium

**Date:** 2025-10-27
**Reporter:** Claude (Graphium Testing)
**Priority:** High - Blocks data listing functionality

---

## Overview

During Graphium web UI testing, three issues were discovered with the EVE library's CouchDB integration. All data **write operations work perfectly**, but read/query operations have API format incompatibilities with CouchDB.

---

## Issue 1: Index Creation Failing ❌

### Problem
CouchDB index creation fails with error: `Missing required key: fields`

### Error Messages
```
Warning: failed to create index containers-status-host:
  CouchDB error (status 400): create_index_failed -
  Bad Request: Missing required key: fields

Warning: failed to create index hosts-datacenter-status:
  CouchDB error (status 400): create_index_failed -
  Bad Request: Missing required key: fields
```

### Graphium Code (Working)
```go
// internal/storage/storage.go:57-67
indexes := []db.Index{
    {
        Name:   "containers-status-host",
        Fields: []string{"@type", "status", "hostedOn"},
        Type:   "json",
    },
    {
        Name:   "hosts-datacenter-status",
        Fields: []string{"@type", "location", "status"},
        Type:   "json",
    },
}

for _, index := range indexes {
    if err := s.service.CreateIndex(index); err != nil {
        fmt.Printf("Warning: failed to create index %s: %v\n", index.Name, err)
    }
}
```

### Expected CouchDB API Format
```json
POST /database/_index
{
  "index": {
    "fields": ["@type", "status", "hostedOn"]
  },
  "name": "containers-status-host",
  "type": "json"
}
```

### What EVE is Currently Sending
Unknown - but CouchDB responds with "Missing required key: fields"

### Fix Needed in EVE
Check the `CreateIndex()` method in `db/couchdb.go` and ensure it's sending the index creation request with this structure:
```json
{
  "index": {
    "fields": ["field1", "field2", "field3"]  // ← This key is missing!
  },
  "name": "index-name",
  "type": "json"
}
```

### Impact
- **Severity:** Low - Indexes are optional for small datasets
- **Performance:** Queries work but may be slower without indexes
- **Functionality:** Non-blocking - data operations work

---

## Issue 2: Query Builder Missing "selector" Key ❌❌

### Problem
CouchDB finder API requires a "selector" key but EVE's QueryBuilder isn't including it

### Error Message
```
CouchDB error (status 400): find_failed -
Bad Request: Missing required key: selector
```

### Graphium Code (Working)
```go
// internal/storage/storage.go:192-206
func (s *Storage) ListContainers(filters map[string]interface{}) ([]*models.Container, error) {
    // Build query with filters
    qb := db.NewQueryBuilder().
        Where("@type", "$eq", "SoftwareApplication")

    // Apply additional filters
    for field, value := range filters {
        qb = qb.And().Where(field, "$eq", value)
    }

    query := qb.Build()

    // Execute query
    containers, err := db.FindTyped[models.Container](s.service, query)
    if err != nil {
        return nil, err  // ← Fails here with "Missing required key: selector"
    }

    // ...
}
```

### Test Case
```bash
# This fails:
$ curl http://localhost:8095/api/v1/containers
{
  "error": "failed to list containers",
  "details": "CouchDB error (status 400): find_failed - Bad Request: Missing required key: selector"
}

# But data IS in CouchDB:
$ curl "http://admin:testpass@localhost:5985/graphium/_all_docs?include_docs=true"
# Returns 3 documents including the container ✅
```

### Expected CouchDB API Format
```json
POST /database/_find
{
  "selector": {
    "@type": {
      "$eq": "SoftwareApplication"
    },
    "status": {
      "$eq": "running"
    }
  },
  "limit": 25
}
```

### What EVE is Currently Sending
The QueryBuilder.Build() is likely returning something like:
```json
{
  "@type": {
    "$eq": "SoftwareApplication"
  }
}
```

But CouchDB needs:
```json
{
  "selector": {    // ← Missing this wrapper!
    "@type": {
      "$eq": "SoftwareApplication"
    }
  }
}
```

### Fix Needed in EVE

**Option 1: Fix QueryBuilder.Build()**
```go
// In db/couchdb_query.go or similar
func (qb *QueryBuilder) Build() map[string]interface{} {
    // Current (broken):
    return qb.conditions

    // Should be:
    return map[string]interface{}{
        "selector": qb.conditions,  // ← Wrap in "selector" key
    }
}
```

**Option 2: Fix FindTyped() function**
```go
// In db/couchdb.go
func FindTyped[T any](service *CouchDBService, query map[string]interface{}) ([]T, error) {
    // Ensure query has "selector" key
    if _, hasSelector := query["selector"]; !hasSelector {
        query = map[string]interface{}{
            "selector": query,  // ← Wrap query in selector if missing
        }
    }

    // Continue with CouchDB _find request...
}
```

### Impact
- **Severity:** HIGH - Blocks all data listing operations
- **Affected:** All FindTyped() calls, all list operations
- **Workaround:** Direct CouchDB access works, only EVE query API affected

---

## Issue 3: Count Methods Failing ❌

### Problem
Count operations return 0 even when documents exist

### Test Case
```bash
# Statistics show wrong counts:
$ curl http://localhost:8095/api/v1/stats
{
  "containerDistribution": {
    "\"host-001\"": 1          # ← This works (from CouchDB view)
  },
  "hostsWithContainers": 1,    # ← This works (from CouchDB view)
  "runningContainers": 0,      # ← WRONG - should be 1
  "totalContainers": 0,        # ← WRONG - should be 1
  "totalHosts": 0              # ← WRONG - should be 1
}

# But documents exist:
$ curl "http://admin:testpass@localhost:5985/graphium/_all_docs"
# Shows 3 documents: _design/graphium, host-001, container-001
```

### Graphium Code
```go
// internal/storage/graph.go:225-237
func (s *Storage) CountContainers(filter map[string]interface{}) (int, error) {
    // Add type filter
    selector := map[string]interface{}{
        "@type": "SoftwareApplication",
    }

    // Merge additional filters
    for k, v := range filter {
        selector[k] = v
    }

    return s.service.Count(selector)  // ← Returns 0 (wrong!)
}
```

### Expected Behavior
- CountContainers(nil) should return 1 (container-001 exists)
- CountHosts(nil) should return 1 (host-001 exists)

### Actual Behavior
- Both return 0

### Likely Root Cause
Same as Issue #2 - the Count() method probably uses the finder API and has the same "missing selector key" problem.

### Fix Needed in EVE
```go
// In db/couchdb.go
func (c *CouchDBService) Count(selector map[string]interface{}) (int, error) {
    // Ensure selector is wrapped correctly for CouchDB _find API
    query := map[string]interface{}{
        "selector": selector,  // ← Add this wrapper
        "limit": 0,            // We only want the count
    }

    // Make request to CouchDB _find endpoint
    // Parse response and return the "warning" field or count from docs
}
```

**Alternative:** Use CouchDB's _all_docs with a view:
```go
func (c *CouchDBService) Count(selector map[string]interface{}) (int, error) {
    // For simple type counts, use a MapReduce view
    // POST /database/_design/graphium/_view/count_by_type?key="SoftwareApplication"
}
```

### Impact
- **Severity:** Medium - Statistics show incorrect values
- **Functionality:** Dashboard shows 0 for counts but other data works
- **Workaround:** CouchDB views partially work (HostContainerCounts works)

---

## Working Features (No Changes Needed) ✅

### 1. Document Creation
```go
s.service.SaveGenericDocument(container)  // ✅ Works perfectly
```

### 2. Document Retrieval by ID
```go
s.service.GetGenericDocument(id, &container)  // ✅ Works perfectly
```

### 3. Document Deletion
```go
s.service.DeleteDocument(id, rev)  // ✅ Works perfectly
```

### 4. CouchDB Views (MapReduce)
```go
// Views work for simple aggregations
containerDistribution map[string]int  // ✅ Works
hostsWithContainers int               // ✅ Works
```

### 5. Database Connection & Info
```go
s.service.GetDatabaseInfo()  // ✅ Works perfectly
```

---

## Summary of Fixes Needed

### Priority 1: QueryBuilder "selector" Wrapper (CRITICAL)
**File:** Likely `db/couchdb_query.go` or `db/couchdb.go`
**Method:** `QueryBuilder.Build()` or `FindTyped()`
**Fix:** Wrap query conditions in `{"selector": {...}}` structure

**Impact:** Fixes all list/find operations

---

### Priority 2: Count Method (HIGH)
**File:** `db/couchdb.go`
**Method:** `Count(selector map[string]interface{})`
**Fix:** Same as Priority 1 - ensure selector is wrapped correctly

**Impact:** Fixes statistics and count operations

---

### Priority 3: Index Creation (MEDIUM)
**File:** `db/couchdb.go`
**Method:** `CreateIndex(index Index)`
**Fix:** Ensure index creation uses `{"index": {"fields": [...]}}` structure

**Impact:** Improves query performance for large datasets

---

## Testing Instructions

### After Fixes Are Applied:

1. **Test Index Creation:**
```bash
# Should succeed without warnings:
go run ./cmd/graphium server --config configs/config.yaml
# Look for: No warnings about index creation
```

2. **Test Containers List:**
```bash
curl http://localhost:8095/api/v1/containers
# Should return:
# {"count": 1, "containers": [{...container-001...}]}
```

3. **Test Hosts List:**
```bash
curl http://localhost:8095/api/v1/hosts
# Should return:
# {"count": 1, "hosts": [{...host-001...}]}
```

4. **Test Statistics:**
```bash
curl http://localhost:8095/api/v1/stats
# Should return:
# {
#   "totalContainers": 1,      // ← Should be 1, not 0
#   "runningContainers": 0,    // ← OK (container status is "running" but query might need adjustment)
#   "totalHosts": 1,           // ← Should be 1, not 0
#   "hostsWithContainers": 1   // ← Already works!
# }
```

5. **Test Web UI:**
```bash
# Open in browser: http://localhost:8095/web/containers
# Should show: Table with nginx-web container listed

# Open: http://localhost:8095/web/hosts
# Should show: Table with web-server-01 host listed
```

---

## CouchDB API Reference

### Finder API (_find)
**Documentation:** https://docs.couchdb.org/en/stable/api/database/find.html

**Correct Request:**
```json
POST /database/_find
{
  "selector": {
    "field": {"$eq": "value"}
  },
  "limit": 25,
  "skip": 0
}
```

### Index API (_index)
**Documentation:** https://docs.couchdb.org/en/stable/api/database/find.html#db-index

**Correct Request:**
```json
POST /database/_index
{
  "index": {
    "fields": ["field1", "field2"]
  },
  "name": "my-index",
  "type": "json"
}
```

---

## Files to Check in EVE

Based on typical CouchDB library structure:

1. `db/couchdb_query.go` - QueryBuilder implementation
2. `db/couchdb.go` - Main service with FindTyped(), Count(), CreateIndex()
3. `db/couchdb_types.go` - Query and Index structures

Look for:
- `type QueryBuilder struct`
- `func (qb *QueryBuilder) Build()`
- `func FindTyped[T any]()`
- `func Count()`
- `func CreateIndex()`

---

## Contact

If you need any clarification or want me to test specific fixes, let me know!

**Testing Environment:**
- CouchDB: 3.3.3 on port 5985
- Credentials: admin/testpass
- Database: graphium
- Test Data: 1 host (host-001), 1 container (container-001)

---

**Reporter:** Claude Code
**Date:** 2025-10-27
**Status:** Awaiting EVE library fixes
