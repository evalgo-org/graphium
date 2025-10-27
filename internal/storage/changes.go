package storage

import (
	"encoding/json"
	"fmt"

	"evalgo.org/graphium/models"
	"eve.evalgo.org/db"
)

// ChangeType represents the type of change that occurred.
type ChangeType string

const (
	ChangeTypeCreated ChangeType = "created"
	ChangeTypeUpdated ChangeType = "updated"
	ChangeTypeDeleted ChangeType = "deleted"
)

// ContainerChange represents a change to a container.
type ContainerChange struct {
	Type      ChangeType
	Container *models.Container
	Sequence  string
}

// HostChange represents a change to a host.
type HostChange struct {
	Type     ChangeType
	Host     *models.Host
	Sequence string
}

// ChangeHandler is a function that handles container or host changes.
type ChangeHandler interface{}

// ContainerChangeHandler handles container changes.
type ContainerChangeHandler func(change ContainerChange)

// HostChangeHandler handles host changes.
type HostChangeHandler func(change HostChange)

// WatchContainerChanges starts listening for container changes in real-time.
// Returns channels for changes and errors, plus a stop function.
func (s *Storage) WatchContainerChanges(handler ContainerChangeHandler) error {
	opts := db.ChangesFeedOptions{
		Since:       "now",
		Feed:        "continuous",
		IncludeDocs: true,
		Heartbeat:   30000, // 30 seconds
		Selector: map[string]interface{}{
			"@type": "SoftwareApplication",
		},
	}

	return s.service.ListenChanges(opts, func(change db.Change) {
		containerChange := s.processContainerChange(change)
		if containerChange != nil {
			handler(*containerChange)
		}
	})
}

// WatchHostChanges starts listening for host changes in real-time.
func (s *Storage) WatchHostChanges(handler HostChangeHandler) error {
	opts := db.ChangesFeedOptions{
		Since:       "now",
		Feed:        "continuous",
		IncludeDocs: true,
		Heartbeat:   30000,
		Selector: map[string]interface{}{
			"@type": "ComputerServer",
		},
	}

	return s.service.ListenChanges(opts, func(change db.Change) {
		hostChange := s.processHostChange(change)
		if hostChange != nil {
			handler(*hostChange)
		}
	})
}

// WatchAllChanges listens for both container and host changes.
func (s *Storage) WatchAllChanges(
	containerHandler ContainerChangeHandler,
	hostHandler HostChangeHandler,
) error {
	opts := db.ChangesFeedOptions{
		Since:       "now",
		Feed:        "continuous",
		IncludeDocs: true,
		Heartbeat:   30000,
	}

	return s.service.ListenChanges(opts, func(change db.Change) {
		// Try to parse as container first
		var doc struct {
			Type string `json:"@type"`
		}

		if err := json.Unmarshal(change.Doc, &doc); err != nil {
			return
		}

		switch doc.Type {
		case "SoftwareApplication":
			if containerHandler != nil {
				containerChange := s.processContainerChange(change)
				if containerChange != nil {
					containerHandler(*containerChange)
				}
			}
		case "ComputerServer":
			if hostHandler != nil {
				hostChange := s.processHostChange(change)
				if hostChange != nil {
					hostHandler(*hostChange)
				}
			}
		}
	})
}

// processContainerChange converts a db.Change to a ContainerChange.
func (s *Storage) processContainerChange(change db.Change) *ContainerChange {
	if change.Deleted {
		return &ContainerChange{
			Type: ChangeTypeDeleted,
			Container: &models.Container{
				ID: change.ID,
			},
			Sequence: change.Seq,
		}
	}

	var container models.Container
	if err := json.Unmarshal(change.Doc, &container); err != nil {
		return nil
	}

	// Determine if it's created or updated
	changeType := ChangeTypeUpdated
	if len(change.Changes) > 0 && change.Changes[0].Rev == container.Rev {
		// If the revision matches and it's the first change, it's likely created
		// This is a heuristic - CouchDB doesn't explicitly tell us if it's created
		changeType = ChangeTypeCreated
	}

	return &ContainerChange{
		Type:      changeType,
		Container: &container,
		Sequence:  change.Seq,
	}
}

// processHostChange converts a db.Change to a HostChange.
func (s *Storage) processHostChange(change db.Change) *HostChange {
	if change.Deleted {
		return &HostChange{
			Type: ChangeTypeDeleted,
			Host: &models.Host{
				ID: change.ID,
			},
			Sequence: change.Seq,
		}
	}

	var host models.Host
	if err := json.Unmarshal(change.Doc, &host); err != nil {
		return nil
	}

	changeType := ChangeTypeUpdated
	if len(change.Changes) > 0 && change.Changes[0].Rev == host.Rev {
		changeType = ChangeTypeCreated
	}

	return &HostChange{
		Type:     changeType,
		Host:     &host,
		Sequence: change.Seq,
	}
}

// GetChangesSince retrieves all changes since a specific sequence.
// This is useful for syncing state when reconnecting after a disconnection.
func (s *Storage) GetChangesSince(sequence string, limit int) ([]db.Change, string, error) {
	opts := db.ChangesFeedOptions{
		Since:       sequence,
		Feed:        "normal",
		IncludeDocs: true,
		Limit:       limit,
	}

	return s.service.GetChanges(opts)
}

// String returns a formatted string representation of the container change.
func (c *ContainerChange) String() string {
	if c.Type == ChangeTypeDeleted {
		return fmt.Sprintf("[%s] Container deleted: %s", c.Type, c.Container.ID)
	}
	return fmt.Sprintf("[%s] Container: %s (%s) - Status: %s",
		c.Type,
		c.Container.Name,
		c.Container.ID,
		c.Container.Status,
	)
}

// String returns a formatted string representation of the host change.
func (h *HostChange) String() string {
	if h.Type == ChangeTypeDeleted {
		return fmt.Sprintf("[%s] Host deleted: %s", h.Type, h.Host.ID)
	}
	return fmt.Sprintf("[%s] Host: %s (%s) - Status: %s",
		h.Type,
		h.Host.Name,
		h.Host.ID,
		h.Host.Status,
	)
}
