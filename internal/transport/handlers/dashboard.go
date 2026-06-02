package handlers

import (
	"embed"
	"html/template"
	"log"

	"github.com/gofiber/fiber/v2"
)

//go:embed templates/dashboard.html
var dashboardHTML embed.FS

// DashboardHandler handles GET /dashboard requests.
type DashboardHandler struct {
	tmpl *template.Template
}

// NewDashboardHandler creates a new DashboardHandler with the embedded template.
func NewDashboardHandler() *DashboardHandler {
	tmpl, err := template.New("dashboard.html").ParseFS(dashboardHTML, "templates/dashboard.html")
	if err != nil {
		log.Fatalf("failed to parse dashboard template: %v", err)
	}
	return &DashboardHandler{tmpl: tmpl}
}

// Handle renders the analytics dashboard HTML page.
func (h *DashboardHandler) Handle(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return h.tmpl.ExecuteTemplate(c, "dashboard.html", nil)
}

