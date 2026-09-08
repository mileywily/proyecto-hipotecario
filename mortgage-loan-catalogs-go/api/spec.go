package api

import _ "embed"

// OpenAPISpec contains the embedded OpenAPI 3.0 specification YAML bytes.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
