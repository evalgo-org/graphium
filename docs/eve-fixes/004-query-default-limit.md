# EVE Fix 004: Add Default Limit to FindTyped

## Issue
`FindTyped` in EVE doesn't set a default limit when querying CouchDB, which causes the underlying Kivik library to use CouchDB's default limit (typically 25-50 documents). This means queries without an explicit limit may not return all matching documents.

## Impact
- Graphium's `ListContainers` was only returning 24 containers out of 109 in the database
- Any query using `FindTyped` without an explicit limit will be silently truncated
- Users may not realize they're missing data

## Root Cause
In `/home/opunix/eve/db/couchdb_query.go`, the `Find` and `FindTyped` functions convert the `MangoQuery` to parameters but don't set a default limit:

```go
// Line 186-208
func (q *MangoQuery) toParams() map[string]interface{} {
	params := make(map[string]interface{})

	if len(q.Fields) > 0 {
		params["fields"] = q.Fields
	}
	if len(q.Sort) > 0 {
		params["sort"] = q.Sort
	}
	if q.Limit > 0 {  // Only sets limit if explicitly provided
		params["limit"] = q.Limit
	}
	// ... rest of function
}
```

## Solution
Add a reasonable default limit (e.g., 10000) to `toParams()` when no limit is specified. This ensures all documents are retrieved unless the user explicitly sets a smaller limit.

## Proposed Fix

In `/home/opunix/eve/db/couchdb_query.go`, modify the `toParams()` method:

```go
func (q *MangoQuery) toParams() map[string]interface{} {
	params := make(map[string]interface{})

	if len(q.Fields) > 0 {
		params["fields"] = q.Fields
	}
	if len(q.Sort) > 0 {
		params["sort"] = q.Sort
	}
	// Set limit - use provided value or default to 10000 to avoid CouchDB's default
	if q.Limit > 0 {
		params["limit"] = q.Limit
	} else {
		// Default to a large limit to retrieve all results
		// Users can override by explicitly setting Limit(n)
		params["limit"] = 10000
	}
	if q.Skip > 0 {
		params["skip"] = q.Skip
	}
	if q.UseIndex != "" {
		params["use_index"] = q.UseIndex
	}

	return params
}
```

## Alternative Solution (Application-Level)
Instead of modifying EVE, Graphium could set explicit limits on all queries:

In `/home/opunix/graphium/internal/storage/storage.go`:
```go
func (s *Storage) ListContainers(filters map[string]interface{}) ([]*models.Container, error) {
	// Build query with filters
	qb := db.NewQueryBuilder().
		Where("@type", "$eq", "SoftwareApplication").
		Limit(10000)  // Add explicit limit

	// Apply additional filters
	for field, value := range filters {
		qb = qb.And().Where(field, "$eq", value)
	}

	query := qb.Build()
	// ... rest of function
}
```

## Recommendation
**Fix EVE library** - This is a library-wide issue that affects all users of `FindTyped`. Setting a reasonable default limit in EVE ensures all applications benefit from the fix without requiring code changes.

## Testing
After applying the fix, verify:
1. `ListContainers` returns all 109 containers
2. Topology and containers page show consistent counts
3. Queries with explicit limits still work correctly
4. Performance remains acceptable with large result sets

## Related Issues
- Graphium Issue: Container count mismatch between topology (109) and containers page (24)
- EVE should document the default behavior or set sensible defaults

## Status
**Pending** - Requires EVE library update

## Workaround
Application code can set explicit limits on all queries:
```go
qb := db.NewQueryBuilder().
	Where(...).
	Limit(10000).  // Explicit limit
	Build()
```
