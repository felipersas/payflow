package http

import "embed"

//go:embed openapi.yaml
var OpenAPISpec embed.FS
