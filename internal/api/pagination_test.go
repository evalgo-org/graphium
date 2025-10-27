package api

import (
	"net/http/httptest"
	"testing"

	"evalgo.org/graphium/models"
	"github.com/labstack/echo/v4"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name        string
		queryParams map[string]string
		wantLimit   int
		wantOffset  int
	}{
		{
			name:        "no parameters - use defaults",
			queryParams: map[string]string{},
			wantLimit:   100,
			wantOffset:  0,
		},
		{
			name: "custom limit and offset",
			queryParams: map[string]string{
				"limit":  "50",
				"offset": "25",
			},
			wantLimit:  50,
			wantOffset: 25,
		},
		{
			name: "limit exceeds max - cap at 1000",
			queryParams: map[string]string{
				"limit": "5000",
			},
			wantLimit:  1000,
			wantOffset: 0,
		},
		{
			name: "negative limit - use default",
			queryParams: map[string]string{
				"limit": "-10",
			},
			wantLimit:  100,
			wantOffset: 0,
		},
		{
			name: "negative offset - use default",
			queryParams: map[string]string{
				"offset": "-5",
			},
			wantLimit:  100,
			wantOffset: 0,
		},
		{
			name: "invalid limit - use default",
			queryParams: map[string]string{
				"limit": "abc",
			},
			wantLimit:  100,
			wantOffset: 0,
		},
		{
			name: "zero limit - use default",
			queryParams: map[string]string{
				"limit": "0",
			},
			wantLimit:  100,
			wantOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest("GET", "/", nil)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			gotLimit, gotOffset := parsePagination(c)

			if gotLimit != tt.wantLimit {
				t.Errorf("parsePagination() limit = %v, want %v", gotLimit, tt.wantLimit)
			}
			if gotOffset != tt.wantOffset {
				t.Errorf("parsePagination() offset = %v, want %v", gotOffset, tt.wantOffset)
			}
		})
	}
}

func TestPaginateSliceContainers(t *testing.T) {
	// Create test containers
	containers := make([]*models.Container, 10)
	for i := 0; i < 10; i++ {
		containers[i] = &models.Container{
			ID:   string(rune('A' + i)),
			Name: "container-" + string(rune('0'+i)),
		}
	}

	tests := []struct {
		name       string
		containers []*models.Container
		limit      int
		offset     int
		wantCount  int
		wantFirst  string
	}{
		{
			name:       "first page",
			containers: containers,
			limit:      5,
			offset:     0,
			wantCount:  5,
			wantFirst:  "A",
		},
		{
			name:       "second page",
			containers: containers,
			limit:      5,
			offset:     5,
			wantCount:  5,
			wantFirst:  "F",
		},
		{
			name:       "partial last page",
			containers: containers,
			limit:      7,
			offset:     7,
			wantCount:  3,
			wantFirst:  "H",
		},
		{
			name:       "offset beyond data",
			containers: containers,
			limit:      5,
			offset:     20,
			wantCount:  0,
			wantFirst:  "",
		},
		{
			name:       "limit exceeds remaining",
			containers: containers,
			limit:      100,
			offset:     8,
			wantCount:  2,
			wantFirst:  "I",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := paginateSliceContainers(tt.containers, tt.limit, tt.offset)

			if len(result) != tt.wantCount {
				t.Errorf("paginateSliceContainers() count = %v, want %v", len(result), tt.wantCount)
			}

			if tt.wantCount > 0 && result[0].ID != tt.wantFirst {
				t.Errorf("paginateSliceContainers() first ID = %v, want %v", result[0].ID, tt.wantFirst)
			}
		})
	}
}

func TestPaginateSliceHosts(t *testing.T) {
	// Create test hosts
	hosts := make([]*models.Host, 10)
	for i := 0; i < 10; i++ {
		hosts[i] = &models.Host{
			ID:   string(rune('A' + i)),
			Name: "host-" + string(rune('0'+i)),
		}
	}

	tests := []struct {
		name      string
		hosts     []*models.Host
		limit     int
		offset    int
		wantCount int
		wantFirst string
	}{
		{
			name:      "first page",
			hosts:     hosts,
			limit:     3,
			offset:    0,
			wantCount: 3,
			wantFirst: "A",
		},
		{
			name:      "middle page",
			hosts:     hosts,
			limit:     3,
			offset:    3,
			wantCount: 3,
			wantFirst: "D",
		},
		{
			name:      "empty result - offset beyond data",
			hosts:     hosts,
			limit:     5,
			offset:    15,
			wantCount: 0,
			wantFirst: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := paginateSliceHosts(tt.hosts, tt.limit, tt.offset)

			if len(result) != tt.wantCount {
				t.Errorf("paginateSliceHosts() count = %v, want %v", len(result), tt.wantCount)
			}

			if tt.wantCount > 0 && result[0].ID != tt.wantFirst {
				t.Errorf("paginateSliceHosts() first ID = %v, want %v", result[0].ID, tt.wantFirst)
			}
		})
	}
}
