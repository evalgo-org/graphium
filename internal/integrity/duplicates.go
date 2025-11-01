package integrity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"eve.evalgo.org/db"
	"github.com/google/uuid"
)

// DocumentMetadata contains minimal information needed for duplicate detection.
type DocumentMetadata struct {
	ID       string                 `json:"_id"`
	Rev      string                 `json:"_rev"`
	Type     string                 `json:"@type"`
	Context  string                 `json:"@context"`
	Data     map[string]interface{} `json:"-"` // Full document data for analysis
	Modified time.Time              // Parsed from dateModified or _rev
}

// scanDuplicates finds documents with duplicate semantic identifiers.
// In CouchDB, true duplicates are actually documents with the same _id but different
// semantic content (e.g., same container ID but from different hosts).
//
// This scans for:
// 1. Documents with matching semantic IDs (like identifier field) but different _id
// 2. Documents that appear to be duplicates based on content similarity
func (s *Service) scanDuplicates(_ context.Context) ([]Issue, error) {
	s.logger.Println("Scanning for duplicate documents...")

	issues := []Issue{}

	// Get all documents from the database using EVE's Find method
	// This returns all documents as raw JSON messages
	allDocs, err := s.db.Find(db.MangoQuery{
		Selector: map[string]interface{}{
			"_id": map[string]interface{}{
				"$gt": nil, // All documents
			},
		},
		Limit: 10000, // Reasonable limit for initial implementation
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all documents: %w", err)
	}

	// Track documents by their semantic identifier
	// Map: semantic_id -> list of document metadata
	semanticIndex := make(map[string][]DocumentMetadata)
	totalDocs := 0

	for _, rawDoc := range allDocs {
		totalDocs++

		var doc map[string]interface{}
		if err := json.Unmarshal(rawDoc, &doc); err != nil {
			s.logger.Printf("Warning: failed to unmarshal document: %v", err)
			continue
		}

		// Extract metadata
		meta := DocumentMetadata{
			Data: doc,
		}

		// Get _id and _rev
		if id, ok := doc["_id"].(string); ok {
			meta.ID = id
		}
		if rev, ok := doc["_rev"].(string); ok {
			meta.Rev = rev
		}

		// Get @type
		if typeVal, ok := doc["@type"].(string); ok {
			meta.Type = typeVal
		}

		// Get @context
		if ctx, ok := doc["@context"].(string); ok {
			meta.Context = ctx
		}

		// Try to parse modification time from dateModified
		if dateModified, ok := doc["dateModified"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, dateModified); err == nil {
				meta.Modified = parsed
			}
		}

		// Extract semantic identifier based on document type
		semanticID := s.extractSemanticID(doc)
		if semanticID != "" {
			semanticIndex[semanticID] = append(semanticIndex[semanticID], meta)
		}
	}

	s.logger.Printf("Scanned %d documents, found %d unique semantic IDs", totalDocs, len(semanticIndex))

	// Find duplicates (semantic IDs with multiple documents)
	for semanticID, docs := range semanticIndex {
		if len(docs) > 1 {
			issue := s.createDuplicateIssue(semanticID, docs)
			issues = append(issues, issue)
		}
	}

	s.logger.Printf("Found %d duplicate document groups", len(issues))

	return issues, nil
}

// extractSemanticID extracts a semantic identifier from a document.
// This is used to identify documents that represent the same real-world entity.
func (s *Service) extractSemanticID(doc map[string]interface{}) string {
	// For containers (SoftwareApplication), use the identifier field
	// which typically contains the Docker container ID
	if typeVal, ok := doc["@type"].(string); ok {
		switch typeVal {
		case "SoftwareApplication":
			// Check for identifier field
			if identifier, ok := doc["identifier"].(string); ok {
				return fmt.Sprintf("container:%s", identifier)
			}
			// Fall back to name if no identifier
			if name, ok := doc["name"].(string); ok {
				return fmt.Sprintf("container:name:%s", name)
			}

		case "ComputerServer":
			// For hosts, use identifier
			if identifier, ok := doc["identifier"].(string); ok {
				return fmt.Sprintf("host:%s", identifier)
			}

		case "Stack":
			// For stacks, use name (stacks are unique by name)
			if name, ok := doc["name"].(string); ok {
				return fmt.Sprintf("stack:%s", name)
			}
		}
	}

	// No semantic ID could be extracted
	return ""
}

// createDuplicateIssue creates an Issue for a set of duplicate documents.
func (s *Service) createDuplicateIssue(semanticID string, docs []DocumentMetadata) Issue {
	issue := Issue{
		ID:          uuid.New().String(),
		Type:        IssueTypeDuplicate,
		Severity:    s.determineDuplicateSeverity(docs),
		Description: fmt.Sprintf("Found %d documents with semantic ID '%s'", len(docs), semanticID),
		DetectedAt:  time.Now(),
		Details: map[string]interface{}{
			"semantic_id":    semanticID,
			"document_count": len(docs),
			"document_ids":   s.extractDocumentIDs(docs),
			"document_types": s.extractDocumentTypes(docs),
		},
	}

	// Use the first document's ID and type for the issue
	if len(docs) > 0 {
		issue.DocumentID = docs[0].ID
		issue.DocumentType = docs[0].Type
	}

	// Create suggested resolution
	issue.SuggestedResolution = s.suggestDuplicateResolution(semanticID, docs)

	return issue
}

// determineDuplicateSeverity determines how critical a duplicate issue is.
func (s *Service) determineDuplicateSeverity(docs []DocumentMetadata) Severity {
	// More duplicates = higher severity
	if len(docs) >= 5 {
		return SeverityCritical
	} else if len(docs) >= 3 {
		return SeverityHigh
	} else {
		return SeverityMedium
	}
}

// extractDocumentIDs extracts all document IDs from a list of metadata.
func (s *Service) extractDocumentIDs(docs []DocumentMetadata) []string {
	ids := make([]string, len(docs))
	for i, doc := range docs {
		ids[i] = doc.ID
	}
	return ids
}

// extractDocumentTypes extracts all unique document types from a list of metadata.
func (s *Service) extractDocumentTypes(docs []DocumentMetadata) []string {
	typeSet := make(map[string]bool)
	for _, doc := range docs {
		if doc.Type != "" {
			typeSet[doc.Type] = true
		}
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// suggestDuplicateResolution creates a suggested resolution for duplicate documents.
func (s *Service) suggestDuplicateResolution(semanticID string, docs []DocumentMetadata) *Resolution {
	// Determine strategy based on document type
	strategy := s.config.Resolution.DefaultStrategy
	if len(docs) > 0 && docs[0].Type != "" {
		if typeStrategy, ok := s.config.Resolution.StrategyByType[docs[0].Type]; ok {
			strategy = typeStrategy
		}
	}

	// Determine risk level
	risk := RiskMedium
	if len(docs) >= 5 {
		risk = RiskHigh // Many duplicates = higher risk
	}

	resolution := &Resolution{
		Strategy: strategy,
		Risk:     risk,
		Description: fmt.Sprintf(
			"Resolve %d duplicate documents for '%s' using %s strategy",
			len(docs), semanticID, strategy,
		),
		Operations:       []RepairOperation{}, // Will be populated when creating repair plan
		RequiresApproval: strategy == StrategyManual || risk == RiskHigh,
	}

	return resolution
}

// selectDocumentToKeep determines which document should be kept when resolving duplicates.
// This implements the "latest_wins" strategy.
func (s *Service) selectDocumentToKeep(docs []DocumentMetadata, strategy ResolutionStrategy) DocumentMetadata {
	if len(docs) == 0 {
		return DocumentMetadata{}
	}

	switch strategy {
	case StrategyLatestWins:
		// Keep the document with the most recent modification time
		latest := docs[0]
		for _, doc := range docs[1:] {
			if doc.Modified.After(latest.Modified) {
				latest = doc
			}
		}
		return latest

	case StrategyHighestRev:
		// Keep the document with the highest revision number
		// Revision format: "N-hash", where N is the revision number
		highest := docs[0]
		highestRev := parseRevisionNumber(docs[0].Rev)
		for _, doc := range docs[1:] {
			rev := parseRevisionNumber(doc.Rev)
			if rev > highestRev {
				highest = doc
				highestRev = rev
			}
		}
		return highest

	default:
		// Default to first document
		return docs[0]
	}
}

// parseRevisionNumber extracts the revision number from a CouchDB revision string.
// Format: "N-hash" -> N
func parseRevisionNumber(rev string) int {
	var revNum int
	fmt.Sscanf(rev, "%d-", &revNum)
	return revNum
}

// mergeDocuments merges multiple duplicate documents into a single document.
// This implements the "merge" strategy.
func (s *Service) mergeDocuments(docs []DocumentMetadata) (map[string]interface{}, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents to merge")
	}

	// Start with the latest document as base
	base := s.selectDocumentToKeep(docs, StrategyLatestWins)
	merged := make(map[string]interface{})

	// Copy base document
	for k, v := range base.Data {
		merged[k] = v
	}

	// Merge data from other documents
	// Only merge fields that are missing or empty in the base
	for _, doc := range docs {
		if doc.ID == base.ID {
			continue // Skip the base document
		}

		for k, v := range doc.Data {
			// Skip system fields
			if k == "_id" || k == "_rev" {
				continue
			}

			// If field doesn't exist in merged doc, add it
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
	}

	// Update metadata
	merged["dateModified"] = time.Now().Format(time.RFC3339)

	return merged, nil
}
