package handlers

import (
	"embed"
	"html/template"
	"log"

	"github.com/gofiber/fiber/v2"
)

//go:embed templates/index.html
var indexHTML embed.FS

// IndexHandler handles GET / requests (main landing page).
type IndexHandler struct {
	tmpl *template.Template
}

// NewIndexHandler creates a new IndexHandler with the embedded template.
func NewIndexHandler() *IndexHandler {
	tmpl, err := template.New("index.html").ParseFS(indexHTML, "templates/index.html")
	if err != nil {
		log.Fatalf("failed to parse index template: %v", err)
	}
	return &IndexHandler{tmpl: tmpl}
}

// Handle renders the main landing page HTML.
func (h *IndexHandler) Handle(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return h.tmpl.ExecuteTemplate(c, "index.html", nil)
}

