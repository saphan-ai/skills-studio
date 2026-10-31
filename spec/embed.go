// Package spec carries the published JSON Schema of the skill format v1, so
// that the verifier checks exactly the file that is published.
package spec

import _ "embed"

// SchemaV1 is spec/skill-format-v1.schema.json, byte for byte.
//
//go:embed skill-format-v1.schema.json
var SchemaV1 []byte
