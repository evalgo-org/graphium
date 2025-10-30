package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// Local type definitions to avoid import cycle with api package
type parseResultResponse struct {
	Valid          bool     `json:"valid"`
	Warnings       []string `json:"warnings"`
	Errors         []string `json:"errors"`
	StackName      string   `json:"stackName,omitempty"`
	ContainerCount int      `json:"containerCount"`
	HasNetwork     bool     `json:"hasNetwork"`
	WaveCount      int      `json:"waveCount"`
}

type deploymentStateResponse struct {
	ID            string                                `json:"id"`
	StackID       string                                `json:"stackId"`
	Status        string                                `json:"status"`
	Phase         string                                `json:"phase,omitempty"`
	Progress      int                                   `json:"progress"`
	Placements    map[string]*models.ContainerPlacement `json:"placements"`
	NetworkInfo   *models.DeployedNetworkInfo           `json:"networkInfo,omitempty"`
	VolumeInfo    map[string]*models.VolumeInfo         `json:"volumeInfo,omitempty"`
	Events        []models.DeploymentEvent              `json:"events,omitempty"`
	StartedAt     time.Time                             `json:"startedAt"`
	CompletedAt   *time.Time                            `json:"completedAt,omitempty"`
	ErrorMessage  string                                `json:"errorMessage,omitempty"`
	RollbackState *models.RollbackState                 `json:"rollbackState,omitempty"`
}

type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
}

// JSONLDDeployPage displays the JSON-LD stack deployment form
func (h *Handler) JSONLDDeployPage(c echo.Context) error {
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}
	return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), ""))
}

// JSONLDValidate validates a JSON-LD stack definition (for HTMX partial)
func (h *Handler) JSONLDValidate(c echo.Context) error {
	stackDefJSON := c.FormValue("stackDefinition")
	if stackDefJSON == "" {
		return c.HTML(http.StatusBadRequest, `<div class="alert alert-error">No stack definition provided</div>`)
	}

	// Parse the JSON
	var definition models.StackDefinition
	if err := json.Unmarshal([]byte(stackDefJSON), &definition); err != nil {
		return c.HTML(http.StatusBadRequest, fmt.Sprintf(`<div class="alert alert-error">Invalid JSON: %s</div>`, err.Error()))
	}

	// Call the API validate endpoint
	validateURL := fmt.Sprintf("http://localhost:%d/api/v1/stacks/jsonld/validate", h.config.Server.Port)
	reqBody, _ := json.Marshal(definition)

	req, err := http.NewRequest("POST", validateURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return c.HTML(http.StatusInternalServerError, `<div class="alert alert-error">Failed to create validation request</div>`)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, fmt.Sprintf(`<div class="alert alert-error">Validation failed: %s</div>`, err.Error()))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result parseResultResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return c.HTML(http.StatusInternalServerError, `<div class="alert alert-error">Failed to parse validation response</div>`)
	}

	// Return HTML fragment
	if result.Valid {
		html := `<div class="alert alert-success">
			<strong>Validation Passed!</strong>
			<ul>
				<li>Stack: ` + result.StackName + `</li>
				<li>Containers: ` + strconv.Itoa(result.ContainerCount) + `</li>
				<li>Waves: ` + strconv.Itoa(result.WaveCount) + `</li>`
		if result.HasNetwork {
			html += `<li>Custom network defined</li>`
		}
		html += `</ul>`
		if len(result.Warnings) > 0 {
			html += `<strong>Warnings:</strong><ul>`
			for _, warning := range result.Warnings {
				html += `<li>` + warning + `</li>`
			}
			html += `</ul>`
		}
		html += `</div>`
		return c.HTML(http.StatusOK, html)
	}

	html := `<div class="alert alert-error">
		<strong>Validation Failed!</strong>
		<ul>`
	for _, err := range result.Errors {
		html += `<li>` + err + `</li>`
	}
	html += `</ul></div>`
	return c.HTML(http.StatusOK, html)
}

// JSONLDDeploy handles the deployment of a JSON-LD stack
func (h *Handler) JSONLDDeploy(c echo.Context) error {
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}

	stackDefJSON := c.FormValue("stackDefinition")
	if stackDefJSON == "" {
		return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), "No stack definition provided"))
	}

	// Parse timeout
	timeout := 300
	if timeoutStr := c.FormValue("timeout"); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = t
		}
	}

	rollbackOnError := c.FormValue("rollbackOnError") == "on"
	pullImages := c.FormValue("pullImages") == "on"

	// Parse the stack definition directly (expecting the full request format)
	var deployReq struct {
		StackDefinition models.StackDefinition `json:"stackDefinition"`
		Timeout         int                     `json:"timeout"`
		RollbackOnError bool                    `json:"rollbackOnError"`
		PullImages      bool                    `json:"pullImages"`
	}

	// Try to parse as full request first
	parseErr := json.Unmarshal([]byte(stackDefJSON), &deployReq)

	// Check if we actually got valid data (not just zero values)
	if parseErr == nil && len(deployReq.StackDefinition.Graph) == 0 {
		// Parsed without error but got empty graph - probably bare JSON-LD, try parsing as StackDefinition
		parseErr = fmt.Errorf("empty graph, retry as bare definition")
	}

	if parseErr != nil {
		// Try parsing just as StackDefinition
		var definition models.StackDefinition
		if err := json.Unmarshal([]byte(stackDefJSON), &definition); err != nil {
			return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), fmt.Sprintf("Invalid JSON: %s", err.Error())))
		}
		c.Logger().Debugf("Parsed as bare definition - Context: %v, Graph len: %d", definition.Context, len(definition.Graph))
		deployReq.StackDefinition = definition
	} else {
		c.Logger().Debugf("Parsed as full request - Context: %v, Graph len: %d", deployReq.StackDefinition.Context, len(deployReq.StackDefinition.Graph))
	}

	// Override with form values
	deployReq.Timeout = timeout
	deployReq.RollbackOnError = rollbackOnError
	deployReq.PullImages = pullImages

	// Call the API deployment endpoint
	deployURL := fmt.Sprintf("http://localhost:%d/api/v1/stacks/jsonld", h.config.Server.Port)
	reqBody, _ := json.Marshal(deployReq)

	// Debug logging
	debugLen := len(reqBody)
	if debugLen > 500 {
		debugLen = 500
	}
	c.Logger().Debugf("Sending deployment request: %s", string(reqBody[:debugLen]))

	req, err := http.NewRequest("POST", deployURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), "Failed to create deployment request"))
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), fmt.Sprintf("Deployment failed: %s", err.Error())))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		var errResp errorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), fmt.Sprintf("Deployment failed: %s - %s", errResp.Message, errResp.Details)))
		}
		return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), fmt.Sprintf("Deployment failed with status %d", resp.StatusCode)))
	}

	var deployResp deploymentStateResponse
	if err := json.Unmarshal(body, &deployResp); err != nil {
		return Render(c, JSONLDDeployPage(user, (*parseResultResponse)(nil), "Failed to parse deployment response"))
	}

	// Redirect to deployment detail page
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/web/stacks/jsonld/deployments/%s", deployResp.ID))
}


// JSONLDDeploymentDetail shows details of a single deployment
func (h *Handler) JSONLDDeploymentDetail(c echo.Context) error {
	var user *models.User
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		user, _ = h.storage.GetUser(claims.UserID)
	}
	id := c.Param("id")

	// Call the API detail endpoint
	detailURL := fmt.Sprintf("http://localhost:%d/api/v1/stacks/jsonld/deployments/%s", h.config.Server.Port, id)

	req, err := http.NewRequest("GET", detailURL, nil)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch deployment")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch deployment: %s", err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return c.String(http.StatusNotFound, "Deployment not found")
	}

	body, _ := io.ReadAll(resp.Body)
	var deployment deploymentStateResponse
	if err := json.Unmarshal(body, &deployment); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to parse deployment response")
	}

	return Render(c, JSONLDDeploymentDetailPage(&deployment, user))
}
