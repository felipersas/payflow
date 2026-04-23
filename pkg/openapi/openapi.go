package openapi

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

//go:embed shared.yaml docs.html
var embeddedFS embed.FS

// LoadSpec reads an OpenAPI YAML spec from the given embed.FS, merges shared
// components from pkg/openapi/shared.yaml, and returns the result as JSON bytes.
// Service-specific components take precedence over shared ones.
func LoadSpec(yamlFS embed.FS) ([]byte, error) {
	raw, err := yamlFS.ReadFile("openapi.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading openapi.yaml: %w", err)
	}

	sharedRaw, err := embeddedFS.ReadFile("shared.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading shared.yaml: %w", err)
	}

	var spec, shared map[string]any
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parsing openapi.yaml: %w", err)
	}
	if err := yaml.Unmarshal(sharedRaw, &shared); err != nil {
		return nil, fmt.Errorf("parsing shared.yaml: %w", err)
	}

	// Merge shared components into spec (spec takes precedence)
	if sharedComp, ok := shared["components"].(map[string]any); ok {
		specComp, _ := spec["components"].(map[string]any)
		if specComp == nil {
			specComp = make(map[string]any)
		}
		for key, val := range sharedComp {
			if existing, ok := specComp[key]; ok {
				// Merge maps (spec values take precedence)
				if existingMap, ok := existing.(map[string]any); ok {
					if valMap, ok := val.(map[string]any); ok {
						merged := make(map[string]any)
						for k, v := range valMap {
							merged[k] = v
						}
						for k, v := range existingMap {
							merged[k] = v
						}
						specComp[key] = merged
						continue
					}
				}
				// Spec value takes precedence, keep it
				continue
			}
			specComp[key] = val
		}
		spec["components"] = specComp
	}

	return json.Marshal(spec)
}

// MustLoadSpec calls LoadSpec and panics on error.
func MustLoadSpec(yamlFS embed.FS) []byte {
	b, err := LoadSpec(yamlFS)
	if err != nil {
		panic(err)
	}
	return b
}

// RegisterDocsRoute adds documentation endpoints to the router:
//   - GET /docs        → Scalar UI (interactive API documentation)
//   - GET /openapi.json → Raw OpenAPI spec in JSON format
func RegisterDocsRoute(r chi.Router, specJSON []byte, serviceName string) {
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		html, err := embeddedFS.ReadFile("docs.html")
		if err != nil {
			http.Error(w, "docs not found", http.StatusInternalServerError)
			return
		}
		page := strings.ReplaceAll(string(html), "{{.ServiceName}}", serviceName)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page))
	})

	r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(specJSON)
	})
}
