// SPDX-License-Identifier: Apache-2.0

package jsonschema

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	jsc "github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

var englishPrinter = message.NewPrinter(language.English)

//go:embed complypack.schema.json
var schemaBytes []byte

var compiledSchema *jsc.Schema

var idPattern *regexp.Regexp
var versionPattern *regexp.Regexp
var evaluatorIDPattern *regexp.Regexp

// IDPattern returns the compiled regex for validating pack IDs
// (reverse-domain notation, 2+ segments). Derived from the embedded
// JSON Schema at init time.
func IDPattern() *regexp.Regexp { return idPattern }

// VersionPattern returns the compiled regex for validating semver
// version strings. Derived from the embedded JSON Schema at init time.
func VersionPattern() *regexp.Regexp { return versionPattern }

// EvaluatorIDPattern returns the compiled regex for validating
// evaluator-id values. Derived from the embedded JSON Schema at init time.
func EvaluatorIDPattern() *regexp.Regexp { return evaluatorIDPattern }

func init() {
	var schemaDoc map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		panic(fmt.Sprintf("unmarshaling schema JSON: %v", err))
	}

	idPattern = extractPattern(schemaDoc, "id")
	versionPattern = extractPattern(schemaDoc, "version")
	evaluatorIDPattern = extractPattern(schemaDoc, "evaluator-id")

	c := jsc.NewCompiler()
	if err := c.AddResource("complypack.schema.json", unmarshalJSON(schemaBytes)); err != nil {
		panic(fmt.Sprintf("adding schema resource: %v", err))
	}
	var err error
	compiledSchema, err = c.Compile("complypack.schema.json")
	if err != nil {
		panic(fmt.Sprintf("compiling complypack schema: %v", err))
	}
}

func extractPattern(schemaDoc map[string]any, field string) *regexp.Regexp {
	props, ok := schemaDoc["properties"].(map[string]any)
	if !ok {
		panic("schema missing properties")
	}
	prop, ok := props[field].(map[string]any)
	if !ok {
		panic(fmt.Sprintf("schema missing property %q", field))
	}
	pattern, ok := prop["pattern"].(string)
	if !ok {
		panic(fmt.Sprintf("schema property %q missing pattern", field))
	}
	return regexp.MustCompile(pattern)
}

func unmarshalJSON(data []byte) any {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		panic(fmt.Sprintf("unmarshaling schema JSON: %v", err))
	}
	return v
}

// ValidateConfig validates YAML config data against the complypack JSON Schema.
// It returns warnings for unknown fields and an error for schema violations.
// When strict is true, unknown-field warnings are promoted to errors.
func ValidateConfig(yamlData []byte, strict bool) ([]string, error) {
	var raw any
	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if raw == nil {
		return nil, nil
	}

	normalized := convertYAMLToJSON(raw)

	if err := compiledSchema.Validate(normalized); err != nil {
		return nil, formatValidationError(err)
	}

	var warnings []string
	if m, ok := normalized.(map[string]any); ok {
		warnings = checkUnknownKeys(m)
	}

	if strict && len(warnings) > 0 {
		return warnings, fmt.Errorf("strict mode: %s", strings.Join(warnings, "; "))
	}

	return warnings, nil
}

// convertYAMLToJSON normalizes YAML-decoded values to JSON-compatible types.
// yaml.Unmarshal produces map[string]any for mappings (Go 1.21+), but
// nested values may still need conversion (e.g., int -> float64 for
// JSON Schema numeric validation).
func convertYAMLToJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[k] = convertYAMLToJSON(v)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[fmt.Sprintf("%v", k)] = convertYAMLToJSON(v)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = convertYAMLToJSON(v)
		}
		return out
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return val
	}
}

// Human-readable error messages keyed by "instancePath/keyword".
// Keep in sync with the patterns in complypack.schema.json.
var friendlyErrors = map[string]string{
	"/id/pattern":           `id must use reverse-domain notation with 2+ segments (e.g. io.complytime.my-pack)`,
	"/id/type":              `id must be a string (e.g. "io.complytime.my-pack")`,
	"/evaluator-id/pattern": `evaluator-id must be a lowercase identifier using letters, digits, hyphens, underscores, or dots (e.g. opa)`,
	"/evaluator-id/type":    `evaluator-id must be a string (e.g. "opa")`,
	"/version/pattern":      `version must be semver (e.g. 1.0.0, 2.1.0-rc.1)`,
	"/version/type":         `version must be a string, not a number (e.g. "1.0.0", not 1)`,
}

func formatValidationError(err error) error {
	ve, ok := err.(*jsc.ValidationError)
	if !ok {
		return err
	}
	var msgs []string
	collectErrors(ve, &msgs)
	return fmt.Errorf("schema validation failed:\n%s", strings.Join(msgs, "\n"))
}

func collectErrors(ve *jsc.ValidationError, msgs *[]string) {
	path := "/" + strings.Join(ve.InstanceLocation, "/")
	keyword := ""
	if kp := ve.ErrorKind.KeywordPath(); len(kp) > 0 {
		keyword = kp[len(kp)-1]
	}

	if len(ve.Causes) == 0 {
		key := path + "/" + keyword
		if friendly, ok := friendlyErrors[key]; ok {
			*msgs = append(*msgs, fmt.Sprintf("  %s: %s", path, friendly))
		} else {
			*msgs = append(*msgs, fmt.Sprintf("  %s: %s", path, ve.ErrorKind.LocalizedString(englishPrinter)))
		}
		return
	}

	for _, cause := range ve.Causes {
		collectErrors(cause, msgs)
	}
}
