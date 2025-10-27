package web

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// Render renders a Templ component to the response.
func Render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response())
}
