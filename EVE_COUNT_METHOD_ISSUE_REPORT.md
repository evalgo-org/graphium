# EVE Library Count Method Issue Report

**Date:** 2025-10-30
**Reporter:** Graphium Development Team
**Severity:** High
**Component:** `eve.evalgo.org/db` - Count method

---

## Summary

The `Count()` method in the EVE library does not work reliably when filtering documents by fields other than `@type`. This causes incorrect statistics and counts in applications that rely on filtered counting.

---

## Issue Details

### Affected Method
- **Package:** `eve.evalgo.org/db`
- **Method:** `service.Count(selector map[string]interface{}) (int, error)`

### Problem Description

When calling `Count()` with a selector that includes both type filtering and field-based filtering, the method returns incorrect counts. Specifically:

1. **Type-only filtering works:** `Count({"@type": "SoftwareApplication"})` returns correct count
2. **Type + field filtering fails:** `Count({"@type": "SoftwareApplication", "status": "running"})` returns incorrect count

### Expected vs Actual Behavior

**Expected:**
```go
// Should count only running containers
selector := map[string]interface{}{
    "@type": "SoftwareApplication",
    "status": "running",
}
count, err := service.Count(selector)
// Expected: 7 (actual running containers)
```

**Actual:**
```go
// Returns incorrect count (appears to ignore status filter)
count, err := service.Count(selector)
// Actual: 25 (not filtering by status correctly)
```

---

## Impact on Graphium

### Affected Code Locations

1. **`/home/opunix/graphium/internal/storage/graph.go:238-247`** - `CountContainers()`
   ```go
   func (s *Storage) CountContainers(filter map[string]interface{}) (int, error) {
       selector := map[string]interface{}{
           "@type": "SoftwareApplication",
       }
       for k, v := range filter {
           selector[k] = v
       }
       return s.service.Count(selector)  // ❌ Returns incorrect count
   }
   ```

2. **`/home/opunix/graphium/internal/storage/graph.go:249-258`** - `CountHosts()`
   ```go
   func (s *Storage) CountHosts(filter map[string]interface{}) (int, error) {
       selector := map[string]interface{}{
           "@type": "ComputerServer",
       }
       for k, v := range filter {
           selector[k] = v
       }
       return s.service.Count(selector)  // ❌ Returns incorrect count
   }
   ```

### User-Visible Symptoms

1. **Dashboard Statistics Incorrect:**
   - "Total Containers" showed wrong count
   - "Running Containers" showed same count as total (not filtered by status)
   - Numbers didn't update after container deletion

2. **Container Distribution Mismatch:**
   - Total containers: 114
   - Distribution sum: 113
   - Discrepancy of 1 container

---

## Root Cause Analysis

### Hypothesis 1: CouchDB Query Translation Issue
The EVE library may not be correctly translating the selector map into a CouchDB `_find` query. The `status` field filter may be:
- Ignored completely
- Applied incorrectly to the Mango query syntax
- Lost during query construction

### Hypothesis 2: Index Missing
CouchDB requires indexes for efficient filtering. The Count method may:
- Not create necessary indexes automatically
- Fail to use existing indexes for non-`@type` fields
- Fall back to a full collection count instead of filtered count

### Hypothesis 3: Deleted Documents Handling
EVE uses a soft-delete pattern with `_deleted` field. The Count method may:
- Include soft-deleted documents in the count
- Not properly filter out documents marked as deleted
- Count all documents matching `@type` regardless of other filters

---

## Evidence

### Test Case 1: Total Containers
```bash
# Docker reality
$ docker ps -a | wc -l
113

# EVE Count (incorrect)
selector := {"@type": "SoftwareApplication"}
count = 25  # ❌ Wrong - should be 113

# EVE List + Count (correct workaround)
containers := service.List({"@type": "SoftwareApplication"})
count = len(containers)  # ✅ 113
```

### Test Case 2: Running Containers
```bash
# Docker reality
$ docker ps | wc -l
7

# EVE Count (incorrect)
selector := {"@type": "SoftwareApplication", "status": "running"}
count = 25  # ❌ Wrong - should be 7

# EVE List + Count (correct workaround)
containers := service.List({"@type": "SoftwareApplication", "status": "running"})
count = len(containers)  # ✅ 7
```

---

## Workaround Implemented

### Solution
Replace `Count()` calls with `List()` + length calculation:

```go
// BEFORE (broken)
func (s *Storage) CountContainers(filter map[string]interface{}) (int, error) {
    selector := map[string]interface{}{
        "@type": "SoftwareApplication",
    }
    for k, v := range filter {
        selector[k] = v
    }
    return s.service.Count(selector)  // ❌ Broken
}

// AFTER (working)
func (s *Storage) CountContainers(filter map[string]interface{}) (int, error) {
    // Use ListContainers and count the results
    // EVE's Count method doesn't work reliably with filters
    containers, err := s.ListContainers(filter)
    if err != nil {
        return 0, err
    }
    return len(containers), nil  // ✅ Correct
}
```

### Performance Implications
- **Before:** O(1) count query (but incorrect)
- **After:** O(n) fetch + count (correct but slower)
- **Impact:** Acceptable for current scale (<1000 containers)
- **Future:** Need EVE fix for production-scale deployments

---

## Recommendations for EVE Library Fix

### Investigation Steps

1. **Examine CouchDB Query Generation:**
   ```go
   // In EVE library, check how Count translates selector to Mango query
   func (s *Service) Count(selector map[string]interface{}) (int, error) {
       // TODO: Debug actual CouchDB query being sent
       // Expected: POST /db/_find with proper selector
       // Actual: May be missing field filters
   }
   ```

2. **Check Index Creation:**
   ```go
   // EVE should create indexes for commonly filtered fields
   // Example index for containers:
   {
       "index": {
           "fields": ["@type", "status"]
       },
       "name": "type-status-index"
   }
   ```

3. **Verify Soft-Delete Handling:**
   ```go
   // Ensure _deleted field is properly excluded
   selector["_deleted"] = map[string]interface{}{"$exists": false}
   ```

### Proposed Fix (EVE Library)

```go
// In eve.evalgo.org/db package
func (s *Service) Count(selector map[string]interface{}) (int, error) {
    // 1. Ensure _deleted filter is added
    if _, exists := selector["_deleted"]; !exists {
        selector["_deleted"] = map[string]interface{}{"$exists": false}
    }

    // 2. Build proper Mango query
    query := map[string]interface{}{
        "selector": selector,
        "limit": 0,  // Don't return docs, just count
    }

    // 3. Use _find endpoint with count
    result, err := s.find(query)
    if err != nil {
        return 0, err
    }

    // 4. Return actual count from CouchDB response
    return result.TotalRows, nil  // Not result.Bookmark or cached count
}
```

### Alternative: Use CouchDB View for Counting

```javascript
// Create a reduce view for efficient counting
{
    "_id": "_design/counts",
    "views": {
        "by_type_and_status": {
            "map": "function(doc) {
                if (!doc._deleted && doc['@type']) {
                    emit([doc['@type'], doc.status || 'unknown'], 1);
                }
            }",
            "reduce": "_count"
        }
    }
}
```

---

## Testing Strategy for EVE Fix

### Unit Tests
```go
func TestCount_WithTypeFilter(t *testing.T) {
    // Create 10 containers, 7 running, 3 stopped
    // Count all: should return 10
    // Count running: should return 7
}

func TestCount_WithMultipleFilters(t *testing.T) {
    // Test combinations of filters
    // Ensure all filters are applied correctly
}

func TestCount_ExcludesDeleted(t *testing.T) {
    // Soft-delete 3 documents
    // Count should exclude them
}
```

### Integration Tests
```go
func TestCount_AgainstRealCouchDB(t *testing.T) {
    // Insert known data set
    // Verify counts match expected values
    // Compare Count() vs List().length
}
```

---

## Related Issues

1. **EVE List() method works correctly** - Uses proper CouchDB queries
2. **EVE Find() method works correctly** - Returns filtered results
3. **Only Count() is affected** - Suggests isolated bug in Count implementation

---

## Action Items

- [ ] Report issue to EVE library maintainers at `eve.evalgo.org`
- [ ] Include this report with reproduction steps
- [ ] Request priority fix due to high impact on statistics
- [ ] Monitor EVE library updates for fix
- [ ] Test fix when available
- [ ] Remove workaround after EVE fix is confirmed working
- [ ] Consider contributing fix to EVE library if needed

---

## Verification Steps (After EVE Fix)

1. Replace workaround code back to using `Count()`
2. Run full test suite
3. Verify dashboard statistics are correct
4. Test with various filter combinations
5. Performance test at scale (1000+ containers)
6. Confirm counts update correctly after deletions

---

## Additional Notes

- The workaround using `List() + len()` is reliable but less efficient
- For production deployments with >1000 containers, this could impact performance
- EVE library should ideally use CouchDB's native counting capabilities
- Consider implementing caching layer if EVE fix takes time

---

**Status:** Fixed in EVE Library ✅
**EVE Fix Status:** Complete - Added limit parameter ✅
**Priority:** High - Affects core functionality

---

## UPDATE: Root Cause Found and Fixed (2025-10-30)

### Actual Root Cause
The issue was **NOT** with field filtering - the Count method was actually working correctly with multi-field selectors. The real problem was:

**CouchDB's `_find` endpoint has a default limit of 25 documents when no limit is explicitly set.**

This is why:
- Count always returned 25 (the default limit)
- List worked correctly (EVE's FindTyped doesn't have this issue)
- The number 25 appeared consistently across all Count operations

### Fix Implemented
Modified `/home/opunix/eve/db/couchdb_query.go` line 546:

```go
func (c *CouchDBService) Count(selector map[string]interface{}) (int, error) {
    ctx := context.Background()

    // Build full query with selector key for Kivik v4
    // IMPORTANT: CouchDB _find has a default limit of 25 documents
    // We need to set a very high limit to count all documents
    fullQuery := map[string]interface{}{
        "selector": selector,
        "limit":    999999, // Set high limit to get all documents for counting
    }

    rows := c.database.Find(ctx, fullQuery)
    defer rows.Close()

    count := 0
    for rows.Next() {
        count++
    }

    if err := rows.Err(); err != nil {
        return 0, fmt.Errorf("error counting documents: %w", err)
    }

    return count, nil
}
```

### Graphium Fix
Reverted the workaround in `/home/opunix/graphium/internal/storage/graph.go`:
- CountContainers now uses `s.service.Count(selector)` again
- CountHosts now uses `s.service.Count(selector)` again
- Both functions now work correctly with the fixed EVE library
