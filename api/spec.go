// Package api provides the OpenAPI specification.
package api

import _ "embed"

// OpenAPISpec is the embedded OpenAPI YAML served at /openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
