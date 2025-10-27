# Eve Library Extension Summary

## 📋 Quick Overview

### Missing Capabilities (11 categories, ~40 functions)

| Category | Priority | Functions Needed |
|----------|----------|------------------|
| **Generic Documents** | 🔴 Critical | 4 generic CRUD functions |
| **View Management** | 🔴 Critical | 3 view functions + types |
| **Mango Queries** | 🔴 Critical | 2 find functions + QueryBuilder |
| **Index Management** | 🔴 Critical | 3 index functions |
| **Graph Traversal** | 🟡 High | 3 traversal functions |
| **Bulk Operations** | 🟡 High | 2 bulk functions |
| **Change Feeds** | 🟡 High | 2 change feed functions |
| **JSON-LD Validation** | 🟢 Medium | 4 validation functions |
| **Database Utilities** | 🟢 Medium | 6 utility functions |
| **Error Handling** | 🟢 Medium | 1 error type + helpers |
| **Configuration** | 🟢 Medium | 1 config struct + constructor |

## 🎯 Top 10 Most Critical Functions

1. **`SaveDocument[T any](doc T)`** - Generic save using Go generics
2. **`GetDocument[T any](id string)`** - Generic retrieval
3. **`CreateDesignDoc(designDoc DesignDoc)`** - Create MapReduce views
4. **`QueryView(design, view string, opts ViewOptions)`** - Query views
5. **`Find(query MangoQuery)`** - Execute complex queries
6. **`CreateIndex(index Index)`** - Create performance indexes
7. **`Traverse(opts TraversalOptions)`** - Follow relationships
8. **`GetDependents(id, relationField string)`** - Reverse lookup
9. **`BulkSaveDocuments(docs []interface{})`** - Batch operations
10. **`ListenChanges(opts, handler func(Change))`** - Real-time updates

## 📊 Implementation Phases

### Phase 1: Foundation (Priority 🔴)
**Functions:** 1-6 from the list above
**Use Case:** Basic storage, views, and queries
**Blocks:** Graphium Phase 2 (Storage Layer)

### Phase 2: Graph Operations (Priority 🟡)
**Functions:** 7-10 from the list above
**Use Case:** Relationship traversal, bulk sync
**Blocks:** Graphium Phase 3 (API Server) & Phase 4 (CLI)

### Phase 3: Production Ready (Priority 🟢)
**Functions:** JSON-LD validation, utilities, error types
**Use Case:** Validation, monitoring, error handling
**Blocks:** Graphium Phase 7 (Testing) & Phase 8 (Production)

## 🔧 Recommended Approach

### Use Go Generics (Requires Go 1.18+):
```go
// Instead of interface{}, use type parameters
func SaveDocument[T any](doc T) (*CouchDBResponse, error)
func GetDocument[T any](id string) (*T, error)
func FindDocuments[T any](query MangoQuery) ([]T, error)
```

### Package Organization:
```
eve.evalgo.org/db/
├── couchdb.go          # Existing (keep for backward compat)
├── couchdb_generic.go  # Generic CRUD
├── couchdb_views.go    # MapReduce views
├── couchdb_query.go    # Mango queries
├── couchdb_graph.go    # Traversal
├── couchdb_changes.go  # Real-time
├── couchdb_jsonld.go   # Validation
├── couchdb_bulk.go     # Bulk ops
└── couchdb_types.go    # Shared types
```

## 📝 Example Usage (After Implementation)

```go
// Graphium usage
import "eve.evalgo.org/db"

// Create service
svc, _ := db.NewCouchDBServiceFromConfig(db.CouchDBConfig{
    URL:      "http://localhost:5984",
    Database: "graphium",
    Username: "admin",
    Password: "password",
})

// Generic operations
container := models.Container{Name: "nginx", Status: "running"}
svc.SaveDocument(container)

// Query with views
results, _ := svc.QueryView("graphium", "containers_by_host", db.ViewOptions{
    Key: "host-123",
})

// Complex queries
containers, _ := svc.FindDocuments[models.Container](db.MangoQuery{
    Selector: map[string]interface{}{
        "status": "running",
        "hostedOn": map[string]interface{}{"$regex": "us-east.*"},
    },
})

// Graph traversal
dependents, _ := svc.GetDependents("postgres-db", "dependsOn")

// Real-time changes
svc.ListenChanges(db.ChangesFeedOptions{
    Feed: "continuous",
    IncludeDocs: true,
}, func(change db.Change) {
    log.Printf("Container updated: %s", change.ID)
})
```

## ⏱️ Estimated Implementation Time

| Phase | Functions | Estimated Time |
|-------|-----------|----------------|
| Phase 1 | ~15 functions | 2-3 days |
| Phase 2 | ~10 functions | 1-2 days |
| Phase 3 | ~15 functions | 1-2 days |
| **Total** | **~40 functions** | **4-7 days** |

## ✅ Benefits for EvalGo Ecosystem

1. **Reusable** - Any project needing graph queries can use this
2. **Tested** - Comprehensive test coverage in eve library
3. **Documented** - Examples and godoc for all functions
4. **Maintained** - Centralized in eve, fixes benefit all projects
5. **Type-safe** - Go generics provide compile-time safety

## 🚀 Next Steps

1. Review `EVE_LIBRARY_REQUIREMENTS.md` for detailed specs
2. Implement functions in eve.evalgo.org/db
3. Tag new version (v0.0.7+)
4. Update Graphium to use new version
5. Implement Graphium storage layer using eve functions
