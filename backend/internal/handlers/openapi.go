package handlers

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openapiSpec []byte

//go:embed openapi_index.html
var openapiHTML []byte

// RedirectOpenAPI redirects /api/openapi to the Swagger UI page.
func RedirectOpenAPI(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/api/openapi/index.html")
}

// ServeOpenAPIIndex serves the Swagger UI HTML page.
func ServeOpenAPIIndex(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", openapiHTML)
}

// ServeOpenAPISpec serves the raw OpenAPI YAML spec.
func ServeOpenAPISpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml", openapiSpec)
}
