package docs

import (
	"embed"

	"github.com/gofiber/fiber/v2"
)

//go:embed openapi.yaml
var specFS embed.FS

const scalarHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Goscratch API Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
    <script id="api-reference" data-url="/docs/openapi.yaml"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

// Module represents the API docs module
type Module struct{}

// NewModule creates a new docs module
func NewModule() *Module {
	return &Module{}
}

// RegisterRoutes registers docs module routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	docs := router.Group("/docs")

	docs.Get("/", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; font-src 'self' https://cdn.jsdelivr.net data:; img-src 'self' data: blob:; connect-src 'self'")
		return c.SendString(scalarHTML)
	})

	docs.Get("/openapi.yaml", func(c *fiber.Ctx) error {
		data, err := specFS.ReadFile("openapi.yaml")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to read OpenAPI spec")
		}
		c.Set("Content-Type", "application/yaml")
		return c.Send(data)
	})
}
