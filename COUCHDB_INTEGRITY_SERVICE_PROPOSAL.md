# CouchDB Integrity Service Proposal

**Date:** 2025-10-30
**Version:** 1.0
**Status:** Proposal
**Priority:** High

---

## Executive Summary

Graphium currently experiences data integrity issues in CouchDB that manifest as:
- Duplicate document entries (same container ID with multiple `_rev` versions)
- 404 errors during agent sync for ~10% of containers
- Inconsistent container counts and statistics
- Revision conflicts during distributed stack deployment

This proposal outlines a comprehensive **CouchDB Integrity Service** that will:
1. Detect and automatically repair duplicate documents
2. Resolve revision conflicts intelligently
3. Validate data consistency continuously
4. Provide repair capabilities with audit logging
5. Monitor database health metrics

---

## Problem Statement

### Current Issues

#### 1. Duplicate Documents
**Symptom:** Same container ID exists multiple times in CouchDB with different `_rev` values

```bash
# Example: Container 80231251a83e exists 3 times
GET /graphium/_all_docs?key="80231251a83e"
{
  "rows": [
    {"id": "80231251a83e", "rev": "1-abc123..."},
    {"id": "80231251a83e", "rev": "2-def456..."},
    {"id": "80231251a83e", "rev": "3-ghi789..."}
  ]
}
```

**Impact:**
- Agent sync fails with 404 errors
- GetContainer returns ambiguous results
- Statistics are incorrect
- UpdateContainer fails intermittently

#### 2. Revision Conflicts
**Symptom:** Stack deployment fails with CouchDB revision conflict errors

```
Error: 409 Conflict - Document update conflict
Context: Deploying stack to multiple hosts simultaneously
```

**Impact:**
- Distributed deployments fail
- Data loss risk during conflict resolution
- Manual intervention required

#### 3. Inconsistent Queries
**Symptom:** Same query returns different results at different times

```go
// Query 1: Returns 113 containers
containers := storage.ListContainers(filter)

// Query 2 (seconds later): Returns 103 containers
count := storage.CountContainers(filter)
```

**Impact:**
- Dashboard shows incorrect statistics
- Monitoring alerts false positive
- User confusion and trust issues

---

## Proposed Solution: CouchDB Integrity Service

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                   Graphium Application                       │
├─────────────────────────────────────────────────────────────┤
│  API Layer  │  Web Layer  │  Agent  │  Orchestrator        │
└──────┬──────┴─────┬───────┴────┬────┴────────┬──────────────┘
       │            │            │              │
       └────────────┴────────────┴──────────────┘
                         │
              ┌──────────▼──────────┐
              │  Storage Layer      │
              │  (/internal/storage)│
              └──────────┬──────────┘
                         │
       ┌─────────────────┴─────────────────┐
       │                                   │
┌──────▼────────────┐          ┌──────────▼──────────┐
│ EVE Library       │          │ Integrity Service   │ ← NEW
│ (Normal Ops)      │          │ (Validation/Repair) │
└─────────┬─────────┘          └──────────┬──────────┘
          │                               │
          └───────────┬───────────────────┘
                      │
            ┌─────────▼──────────┐
            │   CouchDB Server    │
            │   (port 5985)       │
            └─────────────────────┘
```

### Components

#### 1. Integrity Service Core
**Location:** `/internal/integrity/service.go`

**Responsibilities:**
- Detect duplicate documents
- Resolve revision conflicts
- Validate data consistency
- Repair corrupted data
- Generate audit reports

#### 2. Duplicate Detector
**Location:** `/internal/integrity/detector.go`

**Functions:**
- Scan CouchDB for duplicate IDs
- Identify orphaned revisions
- Detect conflicting documents
- Build repair plan

#### 3. Conflict Resolver
**Location:** `/internal/integrity/resolver.go`

**Functions:**
- Analyze revision trees
- Determine canonical version
- Merge conflicting changes
- Preserve data integrity

#### 4. Validator
**Location:** `/internal/integrity/validator.go`

**Functions:**
- Validate document schemas
- Check referential integrity
- Verify index consistency
- Ensure constraint compliance

#### 5. Health Monitor
**Location:** `/internal/integrity/monitor.go`

**Functions:**
- Track database metrics
- Alert on anomalies
- Generate health reports
- Recommend optimizations

---

## Detailed Design

### 1. Duplicate Document Detection

#### Detection Strategy

```go
// Scan strategy: Use _all_docs with conflicts=true
type DuplicateDetector struct {
    service *db.CouchDBService
    logger  *log.Logger
}

func (d *DuplicateDetector) ScanForDuplicates() (*DuplicateReport, error) {
    // Use CouchDB's _changes feed to detect conflicts
    query := map[string]interface{}{
        "conflicts": true,
        "include_docs": true,
    }

    // Scan all documents
    docs, err := d.getAllDocs(query)
    if err != nil {
        return nil, err
    }

    report := &DuplicateReport{
        Timestamp: time.Now(),
        Duplicates: []DuplicateEntry{},
    }

    // Group by ID and find duplicates
    docsByID := make(map[string][]*Document)
    for _, doc := range docs {
        docsByID[doc.ID] = append(docsByID[doc.ID], doc)
    }

    // Identify duplicates (ID appears multiple times)
    for id, versions := range docsByID {
        if len(versions) > 1 {
            report.Duplicates = append(report.Duplicates, DuplicateEntry{
                ID:       id,
                Versions: versions,
                Severity: d.calculateSeverity(versions),
            })
        }
    }

    return report, nil
}
```

#### Duplicate Types

1. **Soft Duplicates** (Revision Conflicts)
   - Same document ID with conflicting revisions
   - CouchDB tracks in `_conflicts` field
   - Resolvable through merge or winner selection

2. **Hard Duplicates** (ID Collision)
   - Same document ID in multiple partition ranges
   - May indicate database corruption
   - Requires manual investigation

3. **Orphaned Revisions**
   - Old revisions not cleaned up by compaction
   - Consume storage space unnecessarily
   - Safe to delete after verification

### 2. Conflict Resolution Strategy

#### Resolution Methods

##### A. Automatic Resolution (Default)
```go
type ConflictResolver struct {
    strategy ResolutionStrategy
}

// ResolutionStrategy determines how conflicts are resolved
type ResolutionStrategy string

const (
    // StrategyLatestWins: Use document with latest timestamp
    StrategyLatestWins ResolutionStrategy = "latest_wins"

    // StrategyHighestRev: Use document with highest _rev number
    StrategyHighestRev ResolutionStrategy = "highest_rev"

    // StrategyMerge: Attempt to merge conflicting fields
    StrategyMerge ResolutionStrategy = "merge"

    // StrategyManual: Flag for human review
    StrategyManual ResolutionStrategy = "manual"
)

func (r *ConflictResolver) ResolveConflict(entry DuplicateEntry) (*Resolution, error) {
    switch r.strategy {
    case StrategyLatestWins:
        return r.resolveByLatestTimestamp(entry)
    case StrategyHighestRev:
        return r.resolveByHighestRevision(entry)
    case StrategyMerge:
        return r.resolveByMerging(entry)
    case StrategyManual:
        return r.flagForManualReview(entry)
    default:
        return nil, fmt.Errorf("unknown resolution strategy: %s", r.strategy)
    }
}
```

##### B. Latest Wins Strategy
```go
func (r *ConflictResolver) resolveByLatestTimestamp(entry DuplicateEntry) (*Resolution, error) {
    // Find document with latest modification time
    var winner *Document
    var latestTime time.Time

    for _, doc := range entry.Versions {
        // Parse timestamp from document metadata
        timestamp, err := parseTimestamp(doc.Metadata)
        if err != nil {
            continue
        }

        if timestamp.After(latestTime) {
            latestTime = timestamp
            winner = doc
        }
    }

    if winner == nil {
        return nil, fmt.Errorf("could not determine winner for %s", entry.ID)
    }

    // Delete all other versions
    losers := []*Document{}
    for _, doc := range entry.Versions {
        if doc.Rev != winner.Rev {
            losers = append(losers, doc)
        }
    }

    return &Resolution{
        Winner:    winner,
        Losers:    losers,
        Strategy:  StrategyLatestWins,
        Timestamp: time.Now(),
    }, nil
}
```

##### C. Merge Strategy (Advanced)
```go
func (r *ConflictResolver) resolveByMerging(entry DuplicateEntry) (*Resolution, error) {
    // Create base document from highest revision
    base := r.findHighestRevision(entry.Versions)
    merged := base.Clone()

    // Merge non-conflicting fields from other versions
    for _, doc := range entry.Versions {
        if doc.Rev == base.Rev {
            continue
        }

        // Merge fields that don't conflict
        for key, value := range doc.Data {
            baseValue, exists := merged.Data[key]

            // If field doesn't exist in base, add it
            if !exists {
                merged.Data[key] = value
                continue
            }

            // If values match, no conflict
            if reflect.DeepEqual(baseValue, value) {
                continue
            }

            // Conflict: apply merge rules
            if mergedValue, ok := r.mergeValues(key, baseValue, value); ok {
                merged.Data[key] = mergedValue
            } else {
                // Can't auto-merge, flag for manual review
                return r.flagForManualReview(entry)
            }
        }
    }

    return &Resolution{
        Winner:    merged,
        Losers:    entry.Versions,
        Strategy:  StrategyMerge,
        Timestamp: time.Now(),
    }, nil
}
```

### 3. Validation Rules

#### Schema Validation
```go
type Validator struct {
    schemas map[string]*jsonschema.Schema
}

func (v *Validator) ValidateDocument(doc *Document) (*ValidationResult, error) {
    result := &ValidationResult{
        Valid:   true,
        Errors:  []ValidationError{},
    }

    // 1. Check required fields
    if err := v.validateRequiredFields(doc); err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, err)
    }

    // 2. Validate JSON-LD @type
    if err := v.validateType(doc); err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, err)
    }

    // 3. Check referential integrity
    if err := v.validateReferences(doc); err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, err)
    }

    // 4. Validate against JSON Schema
    if schema, exists := v.schemas[doc.Type]; exists {
        if err := schema.Validate(doc.Data); err != nil {
            result.Valid = false
            result.Errors = append(result.Errors, ValidationError{
                Field:   "schema",
                Message: err.Error(),
            })
        }
    }

    return result, nil
}
```

#### Referential Integrity
```go
func (v *Validator) validateReferences(doc *Document) error {
    switch doc.Type {
    case "SoftwareApplication": // Container
        return v.validateContainerReferences(doc)
    case "ComputerServer": // Host
        return v.validateHostReferences(doc)
    case "Stack":
        return v.validateStackReferences(doc)
    default:
        return nil
    }
}

func (v *Validator) validateContainerReferences(doc *Document) error {
    // Check if hostedOn (host ID) exists
    if hostID, ok := doc.Data["hostedOn"].(string); ok && hostID != "" {
        exists, err := v.hostExists(hostID)
        if err != nil {
            return fmt.Errorf("error checking host reference: %w", err)
        }
        if !exists {
            return fmt.Errorf("container %s references non-existent host %s",
                doc.ID, hostID)
        }
    }

    // Check if stack reference is valid
    if stackID, ok := doc.Data["stack"].(string); ok && stackID != "" {
        exists, err := v.stackExists(stackID)
        if err != nil {
            return fmt.Errorf("error checking stack reference: %w", err)
        }
        if !exists {
            return fmt.Errorf("container %s references non-existent stack %s",
                doc.ID, stackID)
        }
    }

    return nil
}
```

### 4. Repair Operations

#### Repair Plan
```go
type RepairPlan struct {
    ID          string
    Timestamp   time.Time
    Operations  []RepairOperation
    DryRun      bool
    EstimatedMS int64
}

type RepairOperation struct {
    Type        OperationType
    DocumentID  string
    Action      string
    OldValue    interface{}
    NewValue    interface{}
    Risk        RiskLevel
}

type OperationType string

const (
    OpDeleteDuplicate   OperationType = "delete_duplicate"
    OpResolveConflict   OperationType = "resolve_conflict"
    OpFixReference      OperationType = "fix_reference"
    OpUpdateSchema      OperationType = "update_schema"
    OpCompactDatabase   OperationType = "compact_database"
)

type RiskLevel string

const (
    RiskLow    RiskLevel = "low"     // Safe, no data loss
    RiskMedium RiskLevel = "medium"  // May affect performance
    RiskHigh   RiskLevel = "high"    // Potential data loss
)
```

#### Execution Engine
```go
type RepairExecutor struct {
    service   *db.CouchDBService
    logger    *log.Logger
    audit     *AuditLogger
    dryRun    bool
}

func (e *RepairExecutor) ExecutePlan(plan *RepairPlan) (*RepairResult, error) {
    result := &RepairResult{
        PlanID:    plan.ID,
        StartTime: time.Now(),
        Operations: []OperationResult{},
    }

    // Execute each operation in plan
    for _, op := range plan.Operations {
        opResult := e.executeOperation(op)
        result.Operations = append(result.Operations, opResult)

        // Stop on high-risk failures
        if !opResult.Success && op.Risk == RiskHigh {
            result.Aborted = true
            result.AbortReason = opResult.Error
            break
        }
    }

    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)

    // Write audit log
    e.audit.LogRepair(result)

    return result, nil
}

func (e *RepairExecutor) executeOperation(op RepairOperation) OperationResult {
    if e.dryRun {
        return OperationResult{
            Operation: op,
            Success:   true,
            DryRun:    true,
        }
    }

    result := OperationResult{
        Operation: op,
        StartTime: time.Now(),
    }

    switch op.Type {
    case OpDeleteDuplicate:
        err := e.deleteDuplicate(op)
        result.Success = (err == nil)
        result.Error = err

    case OpResolveConflict:
        err := e.resolveConflict(op)
        result.Success = (err == nil)
        result.Error = err

    case OpFixReference:
        err := e.fixReference(op)
        result.Success = (err == nil)
        result.Error = err

    default:
        result.Success = false
        result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
    }

    result.EndTime = time.Now()
    return result
}
```

### 5. Health Monitoring

#### Metrics Collection
```go
type HealthMonitor struct {
    service *db.CouchDBService
    metrics *MetricsCollector
}

type DatabaseHealth struct {
    Timestamp          time.Time
    TotalDocuments     int
    DuplicateCount     int
    ConflictCount      int
    OrphanedCount      int
    InvalidCount       int
    DatabaseSize       int64
    DiskUsage          float64
    AverageRevisions   float64
    LargestDocument    int64
    RecommendCompaction bool
    HealthScore        int // 0-100
}

func (m *HealthMonitor) CheckHealth() (*DatabaseHealth, error) {
    health := &DatabaseHealth{
        Timestamp: time.Now(),
    }

    // Collect basic stats
    stats, err := m.service.GetDatabaseStats()
    if err != nil {
        return nil, err
    }

    health.TotalDocuments = stats.DocCount
    health.DatabaseSize = stats.DataSize
    health.DiskUsage = stats.DiskSize

    // Scan for issues
    duplicates, _ := m.scanDuplicates()
    health.DuplicateCount = len(duplicates)

    conflicts, _ := m.scanConflicts()
    health.ConflictCount = len(conflicts)

    orphaned, _ := m.scanOrphaned()
    health.OrphanedCount = len(orphaned)

    invalid, _ := m.scanInvalid()
    health.InvalidCount = len(invalid)

    // Calculate health score
    health.HealthScore = m.calculateHealthScore(health)

    // Determine if compaction is needed
    health.RecommendCompaction = m.shouldCompact(stats)

    return health, nil
}

func (m *HealthMonitor) calculateHealthScore(health *DatabaseHealth) int {
    score := 100

    // Deduct for duplicates (5 points per duplicate)
    score -= health.DuplicateCount * 5

    // Deduct for conflicts (3 points per conflict)
    score -= health.ConflictCount * 3

    // Deduct for orphaned docs (1 point per 10 orphaned)
    score -= health.OrphanedCount / 10

    // Deduct for invalid documents (10 points per invalid)
    score -= health.InvalidCount * 10

    // Deduct for high disk usage
    if health.DiskUsage > 0.9 {
        score -= 20
    } else if health.DiskUsage > 0.8 {
        score -= 10
    }

    // Ensure score stays in range [0, 100]
    if score < 0 {
        score = 0
    }

    return score
}
```

---

## Implementation Plan

### Phase 1: Core Infrastructure (Week 1)
**Goal:** Build foundation for integrity service

- [ ] Create `/internal/integrity` package structure
- [ ] Implement `IntegrityService` interface
- [ ] Add configuration options to `config.yaml`
- [ ] Create audit logging system
- [ ] Write unit tests for core types

**Deliverables:**
- Basic service initialization
- Configuration loading
- Audit log structure

### Phase 2: Duplicate Detection (Week 2)
**Goal:** Identify and catalog duplicate documents

- [ ] Implement `DuplicateDetector`
- [ ] Create scanning algorithms
- [ ] Build reporting system
- [ ] Add CLI command: `graphium integrity scan`
- [ ] Write integration tests with test database

**Deliverables:**
- Working duplicate detection
- Scan reports in JSON/text format
- CLI tool for manual scans

### Phase 3: Conflict Resolution (Week 3)
**Goal:** Automatically resolve common conflicts

- [ ] Implement `ConflictResolver`
- [ ] Add resolution strategies (latest wins, highest rev)
- [ ] Create dry-run mode
- [ ] Add CLI command: `graphium integrity resolve`
- [ ] Implement backup before resolution

**Deliverables:**
- Automated conflict resolution
- Multiple resolution strategies
- Safe rollback capability

### Phase 4: Validation Engine (Week 4)
**Goal:** Validate data consistency

- [ ] Implement `Validator`
- [ ] Add schema validation
- [ ] Check referential integrity
- [ ] Create validation reports
- [ ] Add API endpoint: `POST /api/v1/integrity/validate`

**Deliverables:**
- Complete validation engine
- Referential integrity checks
- REST API for validation

### Phase 5: Repair Operations (Week 5)
**Goal:** Provide repair capabilities

- [ ] Implement `RepairExecutor`
- [ ] Create repair plans
- [ ] Add operation tracking
- [ ] Build rollback mechanism
- [ ] Add CLI command: `graphium integrity repair`

**Deliverables:**
- Repair execution engine
- Operation rollback
- Comprehensive audit trail

### Phase 6: Health Monitoring (Week 6)
**Goal:** Continuous health tracking

- [ ] Implement `HealthMonitor`
- [ ] Create metrics dashboard
- [ ] Add alerting system
- [ ] Schedule periodic scans
- [ ] Add web UI for health status

**Deliverables:**
- Real-time health monitoring
- Automated alerts
- Web dashboard

### Phase 7: Integration & Testing (Week 7)
**Goal:** Integrate with existing Graphium components

- [ ] Add integrity checks to agent sync
- [ ] Integrate with storage layer
- [ ] Add background maintenance tasks
- [ ] Comprehensive integration testing
- [ ] Performance benchmarking

**Deliverables:**
- Full integration with Graphium
- Automated maintenance
- Performance metrics

### Phase 8: Documentation & Deployment (Week 8)
**Goal:** Production readiness

- [ ] Write user documentation
- [ ] Create operator guide
- [ ] Add troubleshooting guide
- [ ] Create migration plan
- [ ] Deploy to production

**Deliverables:**
- Complete documentation
- Deployment playbook
- Production monitoring

---

## API Design

### REST Endpoints

```go
// Scan for integrity issues
POST /api/v1/integrity/scan
Request: {
    "types": ["duplicates", "conflicts", "validation"],
    "filters": {
        "@type": "SoftwareApplication"
    }
}
Response: {
    "scan_id": "scan-123",
    "timestamp": "2025-10-30T10:00:00Z",
    "issues_found": 15,
    "report_url": "/api/v1/integrity/reports/scan-123"
}

// Get scan report
GET /api/v1/integrity/reports/:scan_id
Response: {
    "scan_id": "scan-123",
    "timestamp": "2025-10-30T10:00:00Z",
    "duplicates": [...],
    "conflicts": [...],
    "validation_errors": [...]
}

// Create repair plan
POST /api/v1/integrity/repair/plan
Request: {
    "scan_id": "scan-123",
    "strategy": "latest_wins",
    "filters": {
        "risk": ["low", "medium"]
    }
}
Response: {
    "plan_id": "plan-456",
    "operations": 10,
    "estimated_time_ms": 5000,
    "dry_run": true
}

// Execute repair plan
POST /api/v1/integrity/repair/execute/:plan_id
Request: {
    "dry_run": false,
    "confirm": true
}
Response: {
    "execution_id": "exec-789",
    "status": "running",
    "progress_url": "/api/v1/integrity/repair/status/exec-789"
}

// Get health status
GET /api/v1/integrity/health
Response: {
    "timestamp": "2025-10-30T10:00:00Z",
    "health_score": 95,
    "total_documents": 1000,
    "issues": {
        "duplicates": 2,
        "conflicts": 1,
        "invalid": 0
    },
    "recommendations": [
        "Run compaction to reclaim disk space"
    ]
}
```

### CLI Commands

```bash
# Scan for issues
graphium integrity scan [--type duplicates|conflicts|all]

# Generate repair plan
graphium integrity plan --scan scan-123 --strategy latest_wins

# Execute repair (dry run)
graphium integrity repair --plan plan-456 --dry-run

# Execute repair (actual)
graphium integrity repair --plan plan-456 --confirm

# Check health
graphium integrity health

# View reports
graphium integrity reports list
graphium integrity reports show scan-123

# Schedule automatic maintenance
graphium integrity schedule --daily --time 02:00
```

---

## Configuration

### Config File (`configs/config.yaml`)

```yaml
integrity:
  # Enable automatic integrity checks
  enabled: true

  # Scan schedule (cron format)
  scan_schedule: "0 2 * * *"  # Daily at 2 AM

  # Automatic repair settings
  auto_repair:
    enabled: false  # Manual approval required by default
    max_risk: "low"  # Only auto-repair low-risk issues
    strategies:
      - latest_wins

  # Resolution strategies
  resolution:
    default_strategy: "latest_wins"
    conflict_strategies:
      SoftwareApplication: "merge"
      ComputerServer: "latest_wins"
      Stack: "manual"

  # Validation rules
  validation:
    check_references: true
    check_schemas: true
    strict_mode: false

  # Monitoring
  monitoring:
    health_check_interval: "5m"
    alert_threshold: 80  # Alert if health score < 80
    metrics_retention: "30d"

  # Audit logging
  audit:
    enabled: true
    retention: "90d"
    log_path: "/var/log/graphium/integrity/"

  # Performance
  performance:
    max_concurrent_operations: 5
    batch_size: 100
    timeout: "30s"
```

---

## Security Considerations

### 1. Audit Trail
- Log all integrity operations
- Track who initiated repairs
- Record before/after states
- Maintain immutable audit log

### 2. Access Control
- Require admin role for repair operations
- Allow read-only access to health status
- Protect audit logs from tampering
- Encrypt sensitive data in logs

### 3. Data Protection
- Always create backups before repairs
- Implement rollback capability
- Validate operations before execution
- Use dry-run mode by default

### 4. Rate Limiting
- Limit concurrent repair operations
- Throttle API requests
- Prevent resource exhaustion
- Queue large repair jobs

---

## Monitoring & Alerting

### Metrics to Track

```go
// Prometheus metrics
integrity_scans_total{status="success|failure"}
integrity_issues_found{type="duplicate|conflict|invalid"}
integrity_repairs_executed{status="success|failure"}
integrity_health_score gauge
integrity_scan_duration_seconds histogram
integrity_repair_duration_seconds histogram
```

### Alert Rules

```yaml
# Health score critical
- alert: IntegrityHealthCritical
  expr: integrity_health_score < 50
  for: 5m
  annotations:
    summary: "Database integrity health is critical"

# Too many duplicates
- alert: IntegrityDuplicatesHigh
  expr: integrity_issues_found{type="duplicate"} > 100
  for: 10m
  annotations:
    summary: "High number of duplicate documents detected"

# Repair failures
- alert: IntegrityRepairsFailing
  expr: rate(integrity_repairs_executed{status="failure"}[5m]) > 0.1
  for: 5m
  annotations:
    summary: "Integrity repairs are failing"
```

---

## Testing Strategy

### Unit Tests
- Test duplicate detection algorithms
- Validate conflict resolution strategies
- Test validation rules
- Verify repair operations

### Integration Tests
- Test with real CouchDB instance
- Create known duplicate scenarios
- Verify end-to-end repair flow
- Test rollback mechanisms

### Performance Tests
- Benchmark with 10K+ documents
- Test concurrent operations
- Measure scan performance
- Profile memory usage

### Chaos Tests
- Inject database corruption
- Simulate network failures
- Test partial repair failures
- Verify recovery procedures

---

## Migration Strategy

### Phase 1: Audit (Week 1)
1. Deploy integrity service in read-only mode
2. Run comprehensive scans
3. Generate baseline reports
4. Identify critical issues

### Phase 2: Manual Repairs (Week 2-3)
1. Enable dry-run repairs
2. Review repair plans
3. Execute critical repairs manually
4. Validate results

### Phase 3: Automated Monitoring (Week 4)
1. Enable scheduled scans
2. Configure health monitoring
3. Set up alerting
4. Monitor for new issues

### Phase 4: Automated Repairs (Week 5+)
1. Enable auto-repair for low-risk issues
2. Monitor repair success rate
3. Gradually increase automation scope
4. Document lessons learned

---

## Success Metrics

### Primary Metrics
- **Duplicate Count:** Reduce to 0 within 30 days
- **Health Score:** Maintain > 95 continuously
- **Agent Sync Success:** Achieve 100% success rate
- **Conflict Resolution Time:** < 5 minutes automated

### Secondary Metrics
- **Scan Performance:** < 30s for 10K documents
- **Repair Success Rate:** > 99%
- **False Positive Rate:** < 1%
- **System Uptime:** > 99.9%

---

## Risks & Mitigation

### Risk 1: Data Loss During Repair
**Mitigation:**
- Always create backups before operations
- Implement comprehensive rollback
- Use dry-run mode extensively
- Manual approval for high-risk operations

### Risk 2: Performance Impact
**Mitigation:**
- Run scans during off-peak hours
- Throttle concurrent operations
- Use incremental scanning
- Monitor system resources

### Risk 3: False Positives
**Mitigation:**
- Extensive testing before deployment
- Manual review of repair plans
- Gradual rollout with monitoring
- Easy rollback procedures

### Risk 4: Complexity
**Mitigation:**
- Phased implementation
- Comprehensive documentation
- Extensive logging
- Simple CLI interface

---

## Open Questions

1. **Should we support distributed repair coordination?**
   - If multiple Graphium instances run integrity checks simultaneously
   - Need distributed locking mechanism?

2. **How to handle very large databases (1M+ documents)?**
   - Incremental scanning?
   - Partitioned repairs?
   - Separate repair service?

3. **Should repairs be idempotent?**
   - Can we safely re-run repair operations?
   - How to detect already-repaired documents?

4. **What's the retention policy for audit logs?**
   - Keep all repairs forever?
   - Archive old logs?
   - Compliance requirements?

---

## Alternatives Considered

### Alternative 1: CouchDB Compaction Only
**Pros:** Simple, built-in solution
**Cons:** Doesn't resolve conflicts, doesn't detect duplicates
**Decision:** Insufficient - need custom solution

### Alternative 2: External Tool (e.g., couchdb-fauxton)
**Pros:** Existing tool, well-tested
**Cons:** No automation, no integration, manual process
**Decision:** Rejected - need automated solution

### Alternative 3: Migration to Different Database
**Pros:** Avoid CouchDB-specific issues
**Cons:** Major architectural change, high risk
**Decision:** Rejected - fix current system first

---

## References

- [CouchDB Conflict Management](https://docs.couchdb.org/en/stable/replication/conflicts.html)
- [CouchDB Compaction](https://docs.couchdb.org/en/stable/maintenance/compaction.html)
- [EVE Library Documentation](https://eve.evalgo.org)
- [Graphium Architecture](./ARCHITECTURE.md)

---

## Appendix A: Duplicate Examples

### Example 1: Revision Conflict
```json
{
  "_id": "container-123",
  "_rev": "2-abc",
  "_conflicts": ["1-def"],
  "@type": "SoftwareApplication",
  "name": "nginx",
  "status": "running"
}
```

### Example 2: Multiple Documents
```bash
# Document 1
{"_id": "container-123", "_rev": "1-abc", "status": "stopped"}

# Document 2 (duplicate ID)
{"_id": "container-123", "_rev": "1-def", "status": "running"}
```

---

## Appendix B: Resolution Decision Tree

```
Document Conflict Detected
│
├─> Are timestamps available?
│   ├─> YES: Use StrategyLatestWins
│   └─> NO: Continue to next check
│
├─> Are revisions comparable?
│   ├─> YES: Use StrategyHighestRev
│   └─> NO: Continue to next check
│
├─> Are fields mergeable?
│   ├─> YES: Use StrategyMerge
│   └─> NO: Use StrategyManual
```

---

**Approval Required:**
- [ ] Architecture Team
- [ ] DevOps Team
- [ ] Security Team
- [ ] Product Owner

**Estimated Effort:** 8 weeks (1 senior engineer)
**Priority:** P0 - Critical for production stability
