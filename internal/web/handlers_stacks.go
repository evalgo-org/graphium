package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// paginateStacks returns a slice of stacks for the current page
func paginateStacks(stacks []*models.Stack, page, pageSize int) []*models.Stack {
	start := (page - 1) * pageSize
	if start >= len(stacks) {
		return []*models.Stack{}
	}
	end := start + pageSize
	if end > len(stacks) {
		end = len(stacks)
	}
	return stacks[start:end]
}

// StacksList renders the stacks list page.
func (h *Handler) StacksList(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	// Get filters from query params
	filters := make(map[string]interface{})
	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["datacenter"] = datacenter
	}

	// Get page number from query params (default to 1)
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get stacks
	allStacks, err := h.storage.ListStacks(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load stacks")
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allStacks), page, pageSize)
	stacks := paginateStacks(allStacks, page, pageSize)

	return Render(c, StacksListWithUser(stacks, pagination, user))
}

// StacksTable renders just the stacks table (for HTMX).
func (h *Handler) StacksTable(c echo.Context) error {
	// Get filters from query params
	filters := make(map[string]interface{})
	queryParts := []string{}

	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
		queryParts = append(queryParts, "status="+status)
	}
	if datacenter := c.QueryParam("datacenter"); datacenter != "" {
		filters["datacenter"] = datacenter
		queryParts = append(queryParts, "datacenter="+datacenter)
	}

	// Get search query parameter
	search := c.QueryParam("search")
	if search != "" {
		queryParts = append(queryParts, "search="+search)
	}

	queryString := ""
	if len(queryParts) > 0 {
		queryString = strings.Join(queryParts, "&")
	}

	// Get page number from query params (default to 1)
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get stacks
	allStacks, err := h.storage.ListStacks(filters)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load stacks")
	}

	// Apply search filter if present (client-side filtering by name)
	if search != "" {
		filteredStacks := make([]*models.Stack, 0)
		searchLower := strings.ToLower(search)
		for _, stack := range allStacks {
			if strings.Contains(strings.ToLower(stack.Name), searchLower) ||
				strings.Contains(strings.ToLower(stack.ID), searchLower) {
				filteredStacks = append(filteredStacks, stack)
			}
		}
		allStacks = filteredStacks
	}

	// Calculate pagination
	pageSize := 10
	pagination := calculatePagination(len(allStacks), page, pageSize)
	stacks := paginateStacks(allStacks, page, pageSize)

	return Render(c, StacksTableWithPagination(stacks, pagination, queryString))
}

// StackDetail renders the stack detail page.
func (h *Handler) StackDetail(c echo.Context) error {
	// Get current user from context (if authenticated)
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "Stack ID is required")
	}

	// Get stack
	stack, err := h.storage.GetStack(id)
	if err != nil {
		return c.String(http.StatusNotFound, "Stack not found")
	}

	// Get deployment info if stack is deployed
	var deployment *models.StackDeployment
	if stack.Status == "running" || stack.Status == "stopped" {
		deployment, _ = h.storage.GetDeployment(id)
		// Ignore error - deployment might not exist
	}

	return Render(c, StackDetailWithUser(stack, deployment, user))
}
