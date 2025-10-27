# Eve Library Extension Requirements for Graphium

This document outlines the capabilities that need to be added to `eve.evalgo.org/db` to support Graphium's graph-based container orchestration platform.

## Current State Analysis

### ✅ What Eve Already Provides:
- Basic CRUD operations (Save, Get, Delete)
- Simple state-based filtering (`GetDocumentsByState`)
- Connection management (`NewCouchDBService`)
- CouchDB client wrapper using Kivik

### ❌ What's Missing for Graphium:

---

## 1. Generic Document Support

**Problem:** Eve currently uses `FlowProcessDocument` type, but Graphium needs to work with any document type (Container, Host, Network, Volume, etc.).

**Required Functions:**

```go
// Generic document operations that work with any struct
func (c *CouchDBService) SaveGenericDocument(doc interface{}) (*CouchDBResponse, error)
func (c *CouchDBService) GetGenericDocument(id string, result interface{}) error
func (c *CouchDBService) DeleteGenericDocument(id, rev string) error
func (c *CouchDBService) GetAllGenericDocuments(docType string, result interface{}) error
```

**Alternative Approach:** Use Go generics (Go 1.18+)
```go
func SaveDocument[T any](doc T) (*CouchDBResponse, error)
func GetDocument[T any](id string) (*T, error)
func GetDocumentsByType[T any](docType string) ([]T, error)
```

---

## 2. CouchDB View Management

**Problem:** Graph queries require MapReduce views, but Eve doesn't provide view creation/management.

**Required Functions:**

```go
// View represents a CouchDB MapReduce view
type View struct {
    Name   string
    Map    string // JavaScript map function
    Reduce string // JavaScript reduce function (optional)
}

// DesignDoc represents a CouchDB design document containing views
type DesignDoc struct {
    ID       string
    Language string
    Views    map[string]View
}

// Create or update a design document with views
func (c *CouchDBService) CreateDesignDoc(designDoc DesignDoc) error

// Query a view with optional parameters
func (c *CouchDBService) QueryView(designName, viewName string, opts ViewOptions) (*ViewResult, error)

// ViewOptions for querying views
type ViewOptions struct {
    Key         interface{}
    StartKey    interface{}
    EndKey      interface{}
    IncludeDocs bool
    Limit       int
    Skip        int
    Descending  bool
    Group       bool
    GroupLevel  int
    Reduce      bool
}

// ViewResult contains the results from a view query
type ViewResult struct {
    TotalRows int
    Offset    int
    Rows      []ViewRow
}

type ViewRow struct {
    ID    string
    Key   interface{}
    Value interface{}
    Doc   json.RawMessage // Raw document if IncludeDocs=true
}
```

**Example Views Needed for Graphium:**

```javascript
// View: containers_by_host
{
  "map": "function(doc) { if (doc['@type'] === 'SoftwareApplication' && doc.hostedOn) { emit(doc.hostedOn, doc); } }"
}

// View: hosts_by_datacenter
{
  "map": "function(doc) { if (doc['@type'] === 'ComputerServer' && doc.location) { emit(doc.location, doc); } }"
}

// View: containers_by_status
{
  "map": "function(doc) { if (doc['@type'] === 'SoftwareApplication') { emit(doc.status, doc); } }"
}
```

---

## 3. Graph Traversal Functions

**Problem:** Need to traverse relationships between entities (containers → hosts → datacenters).

**Required Functions:**

```go
// Traverse follows relationships from a starting document
type TraversalOptions struct {
    StartID       string   // Starting document ID
    Depth         int      // How many levels deep to traverse
    RelationField string   // Field name containing the relationship (e.g., "hostedOn")
    Direction     string   // "forward" or "reverse"
    Filter        map[string]interface{} // Optional filters
}

func (c *CouchDBService) Traverse(opts TraversalOptions) ([]json.RawMessage, error)

// TraverseTyped returns strongly-typed results using generics
func TraverseTyped[T any](opts TraversalOptions) ([]T, error)

// GetDependents finds all documents that reference the given ID
// Example: Find all containers running on host-123
func (c *CouchDBService) GetDependents(id string, relationField string) ([]json.RawMessage, error)

// GetDependencies finds all documents referenced by the given document
// Example: Find the host that container-456 runs on
func (c *CouchDBService) GetDependencies(id string, relationFields []string) (map[string]json.RawMessage, error)
```

---

## 4. Complex Query Support (Mango Queries)

**Problem:** Need complex filtering with multiple conditions (--where clauses).

**Required Functions:**

```go
// MangoQuery represents a CouchDB Mango query
type MangoQuery struct {
    Selector map[string]interface{} // MongoDB-style selectors
    Fields   []string               // Fields to return
    Sort     []map[string]string    // Sort order
    Limit    int                    // Max results
    Skip     int                    // Offset
    UseIndex string                 // Index hint
}

// Find executes a Mango query
func (c *CouchDBService) Find(query MangoQuery) ([]json.RawMessage, error)

// FindTyped executes a Mango query with typed results
func FindTyped[T any](query MangoQuery) ([]T, error)

// Example usage:
// query := MangoQuery{
//     Selector: map[string]interface{}{
//         "$and": []interface{}{
//             map[string]interface{}{"status": "running"},
//             map[string]interface{}{"hostedOn": map[string]interface{}{"$regex": "us-east.*"}},
//         },
//     },
//     Limit: 100,
// }
```

**Helper Functions for Query Building:**

```go
// QueryBuilder helps construct complex Mango queries
type QueryBuilder struct {
    conditions []map[string]interface{}
    fields     []string
    limit      int
    skip       int
}

func NewQueryBuilder() *QueryBuilder
func (qb *QueryBuilder) Where(field string, operator string, value interface{}) *QueryBuilder
func (qb *QueryBuilder) And() *QueryBuilder
func (qb *QueryBuilder) Or() *QueryBuilder
func (qb *QueryBuilder) Select(fields ...string) *QueryBuilder
func (qb *QueryBuilder) Limit(n int) *QueryBuilder
func (qb *QueryBuilder) Skip(n int) *QueryBuilder
func (qb *QueryBuilder) Build() MangoQuery

// Example:
// query := NewQueryBuilder().
//     Where("status", "=", "running").
//     And().
//     Where("location", "=", "us-east").
//     Limit(50).
//     Build()
```

---

## 5. Index Management

**Problem:** Performance optimization requires creating indexes for frequently queried fields.

**Required Functions:**

```go
// Index represents a CouchDB index
type Index struct {
    Name   string
    Fields []string
    Type   string // "json" or "text"
}

// CreateIndex creates a new index
func (c *CouchDBService) CreateIndex(index Index) error

// ListIndexes returns all indexes for the database
func (c *CouchDBService) ListIndexes() ([]Index, error)

// DeleteIndex removes an index
func (c *CouchDBService) DeleteIndex(designDoc, name string) error
```

---

## 6. Bulk Operations

**Problem:** Need efficient batch operations for syncing multiple containers/hosts.

**Required Functions:**

```go
// BulkSaveDocuments saves multiple documents in a single request
func (c *CouchDBService) BulkSaveDocuments(docs []interface{}) ([]BulkResult, error)

// BulkDeleteDocuments deletes multiple documents
func (c *CouchDBService) BulkDeleteDocuments(docs []BulkDeleteDoc) ([]BulkResult, error)

type BulkDeleteDoc struct {
    ID      string `json:"_id"`
    Rev     string `json:"_rev"`
    Deleted bool   `json:"_deleted"`
}

type BulkResult struct {
    ID     string `json:"id"`
    Rev    string `json:"rev,omitempty"`
    Error  string `json:"error,omitempty"`
    Reason string `json:"reason,omitempty"`
    OK     bool   `json:"ok"`
}
```

---

## 7. Change Feeds / Real-Time Updates

**Problem:** WebSocket support requires listening to database changes.

**Required Functions:**

```go
// ChangesFeedOptions configures the changes feed
type ChangesFeedOptions struct {
    Since       string              // Sequence to start from ("now", "0", or sequence ID)
    Feed        string              // "normal", "longpoll", "continuous"
    Filter      string              // Filter function name
    IncludeDocs bool                // Include full documents
    Heartbeat   int                 // Milliseconds between heartbeats
    Timeout     int                 // Request timeout
    Limit       int                 // Max changes to return
    Descending  bool                // Reverse order
    Selector    map[string]interface{} // Mango selector for filtering
}

// Change represents a single change event
type Change struct {
    Seq     string          `json:"seq"`
    ID      string          `json:"id"`
    Changes []ChangeRev     `json:"changes"`
    Deleted bool            `json:"deleted,omitempty"`
    Doc     json.RawMessage `json:"doc,omitempty"`
}

type ChangeRev struct {
    Rev string `json:"rev"`
}

// ListenChanges starts listening to database changes
func (c *CouchDBService) ListenChanges(opts ChangesFeedOptions, handler func(Change)) error

// GetChanges gets changes without continuous listening
func (c *CouchDBService) GetChanges(opts ChangesFeedOptions) ([]Change, string, error)
```

---

## 8. JSON-LD Validation Support

**Problem:** Graphium uses JSON-LD for semantic data, need validation helpers.

**Required Functions:**

```go
// ValidateJSONLD validates a document against JSON-LD schema
func ValidateJSONLD(doc interface{}, context string) error

// ExpandJSONLD expands a JSON-LD document
func ExpandJSONLD(doc interface{}) (map[string]interface{}, error)

// CompactJSONLD compacts a JSON-LD document with a context
func CompactJSONLD(doc interface{}, context string) (map[string]interface{}, error)

// NormalizeJSONLD normalizes a JSON-LD document (RDF canonicalization)
func NormalizeJSONLD(doc interface{}) (string, error)
```

---

## 9. Database Utilities

**Problem:** Need database management functions.

**Required Functions:**

```go
// CreateDatabase creates a new database if it doesn't exist
func CreateDatabase(url, dbName string) error

// DeleteDatabase deletes a database
func DeleteDatabase(url, dbName string) error

// DatabaseExists checks if a database exists
func DatabaseExists(url, dbName string) (bool, error)

// GetDatabaseInfo returns database metadata
func (c *CouchDBService) GetDatabaseInfo() (*DatabaseInfo, error)

type DatabaseInfo struct {
    DBName          string `json:"db_name"`
    DocCount        int64  `json:"doc_count"`
    DocDelCount     int64  `json:"doc_del_count"`
    UpdateSeq       string `json:"update_seq"`
    PurgeSeq        int64  `json:"purge_seq"`
    CompactRunning  bool   `json:"compact_running"`
    DiskSize        int64  `json:"disk_size"`
    DataSize        int64  `json:"data_size"`
    InstanceStartTime string `json:"instance_start_time"`
}

// CompactDatabase triggers database compaction
func (c *CouchDBService) CompactDatabase() error
```

---

## 10. Error Handling Improvements

**Problem:** Need better error types for different CouchDB errors.

**Required Types:**

```go
// CouchDBError represents a CouchDB-specific error
type CouchDBError struct {
    StatusCode int
    Error      string
    Reason     string
}

func (e *CouchDBError) Error() string
func (e *CouchDBError) IsConflict() bool
func (e *CouchDBError) IsNotFound() bool
func (e *CouchDBError) IsUnauthorized() bool
```

---

## 11. Configuration Extensions

**Problem:** Current config is flow-specific, need generic CouchDB config.

**Required:**

```go
// CouchDBConfig for generic CouchDB connections
type CouchDBConfig struct {
    URL             string
    Database        string
    Username        string
    Password        string
    MaxConnections  int
    Timeout         int
    CreateIfMissing bool
    TLS             *TLSConfig
}

type TLSConfig struct {
    Enabled            bool
    CertFile           string
    KeyFile            string
    CAFile             string
    InsecureSkipVerify bool
}

// NewCouchDBServiceFromConfig creates service from config
func NewCouchDBServiceFromConfig(config CouchDBConfig) (*CouchDBService, error)
```

---

## Summary of Required Additions

### Priority 1 (Critical for Phase 2):
1. ✅ Generic document support (generics or interface{})
2. ✅ CouchDB view management (CreateDesignDoc, QueryView)
3. ✅ Mango query support (Find, QueryBuilder)
4. ✅ Index management (CreateIndex)

### Priority 2 (Critical for Phase 3):
5. ✅ Graph traversal (Traverse, GetDependents)
6. ✅ Bulk operations (BulkSave, BulkDelete)
7. ✅ Change feeds (ListenChanges for WebSocket)

### Priority 3 (Important for Production):
8. ✅ JSON-LD validation helpers
9. ✅ Database utilities (CreateDatabase, GetDatabaseInfo)
10. ✅ Better error handling (CouchDBError types)
11. ✅ Generic configuration (CouchDBConfig)

---

## Implementation Notes

### Suggested Package Structure in Eve:
```
eve.evalgo.org/db/
├── couchdb.go          # Current flow-specific code
├── couchdb_generic.go  # New generic document operations
├── couchdb_views.go    # View management
├── couchdb_query.go    # Mango queries and query builder
├── couchdb_graph.go    # Graph traversal functions
├── couchdb_changes.go  # Change feed support
├── couchdb_jsonld.go   # JSON-LD validation
├── couchdb_bulk.go     # Bulk operations
└── couchdb_types.go    # Common types and errors
```

### Backward Compatibility:
- Keep existing `FlowProcessDocument` functions
- Add new generic functions alongside
- Use Go generics where possible (requires Go 1.18+)

### Testing Requirements:
- Integration tests with real CouchDB instance
- Mock tests for graph traversal logic
- Benchmark tests for bulk operations
- Example code for each new function

---

## Next Steps

Once these capabilities are added to eve.evalgo.org/db@v0.0.7+:
1. Graphium can use the library directly
2. No custom storage wrapper needed
3. Better code reuse across EvalGo projects
4. Comprehensive test coverage from eve library
