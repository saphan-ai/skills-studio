package skillfmt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"unicode/utf8"
)

// Schema is a JSON Schema (2020-12) restricted to the keywords the published
// skill format schema uses. Compile refuses a schema that uses any other
// keyword, so a keyword the validator would silently skip cannot hide in it.
type Schema struct {
	root any
}

// annotations are keywords that never affect validation.
var annotations = map[string]bool{
	"$schema": true, "$id": true, "title": true, "description": true, "$comment": true, "x-rule": true,
}

// Compile reads schema JSON and checks every keyword in it is supported.
func Compile(data []byte) (*Schema, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("schema is not JSON: %w", err)
	}
	if err := check(root, "#"); err != nil {
		return nil, err
	}
	return &Schema{root: root}, nil
}

func check(node any, at string) error {
	switch n := node.(type) {
	case bool:
		return nil
	case map[string]any:
		for k, v := range n {
			switch k {
			case "type", "required", "enum":
			case "pattern":
				s, ok := v.(string)
				if !ok {
					return fmt.Errorf("%s/pattern is not a string", at)
				}
				if _, err := regexp.Compile(s); err != nil {
					return fmt.Errorf("%s/pattern: %w", at, err)
				}
			case "minLength", "maxLength":
				if _, ok := v.(float64); !ok {
					return fmt.Errorf("%s/%s is not a number", at, k)
				}
			case "properties":
				props, ok := v.(map[string]any)
				if !ok {
					return fmt.Errorf("%s/properties is not an object", at)
				}
				for pk, pv := range props {
					if err := check(pv, at+"/properties/"+pk); err != nil {
						return err
					}
				}
			case "additionalProperties":
				if err := check(v, at+"/additionalProperties"); err != nil {
					return err
				}
			case "allOf":
				list, ok := v.([]any)
				if !ok {
					return fmt.Errorf("%s/allOf is not an array", at)
				}
				for i, sub := range list {
					if err := check(sub, fmt.Sprintf("%s/allOf/%d", at, i)); err != nil {
						return err
					}
				}
			default:
				if !annotations[k] {
					return fmt.Errorf("%s: keyword %q is not supported by this validator", at, k)
				}
			}
		}
		return nil
	}
	return fmt.Errorf("%s is neither an object nor a boolean", at)
}

// Validate checks a document in the JSON data model against the schema and
// returns every failure, each carrying the x-rule of the sub-schema that failed
// (RuleSchemaUnattributed when that sub-schema names none).
func (s *Schema) Validate(doc any) []Violation {
	var out []Violation
	validate(s.root, doc, "", RuleSchemaUnattributed, &out)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Rule < out[j].Rule })
	return out
}

func fail(out *[]Violation, rule, path, format string, a ...any) {
	if path == "" {
		path = "/"
	}
	*out = append(*out, Violation{Rule: rule, Path: path, Message: fmt.Sprintf(format, a...)})
}

func validate(node, v any, path, rule string, out *[]Violation) {
	switch n := node.(type) {
	case bool:
		if !n {
			fail(out, rule, path, "no value is allowed here")
		}
		return
	case map[string]any:
		if r, ok := n["x-rule"].(string); ok {
			rule = r
		}
		if t, ok := n["type"].(string); ok && !isType(v, t) {
			fail(out, rule, path, "expected %s, found %s", t, typeName(v))
		}
		if list, ok := n["enum"].([]any); ok && !inEnum(list, v) {
			fail(out, rule, path, "%v is not one of %v", v, list)
		}
		if str, ok := v.(string); ok {
			length := utf8.RuneCountInString(str)
			if pat, ok := n["pattern"].(string); ok && !regexp.MustCompile(pat).MatchString(str) {
				fail(out, rule, path, "%q does not match %s", str, pat)
			}
			if m, ok := n["minLength"].(float64); ok && float64(length) < m {
				fail(out, rule, path, "length %d is below %d", length, int(m))
			}
			if m, ok := n["maxLength"].(float64); ok && float64(length) > m {
				fail(out, rule, path, "length %d is above %d", length, int(m))
			}
		}
		if obj, ok := v.(map[string]any); ok {
			if req, ok := n["required"].([]any); ok {
				for _, r := range req {
					if _, present := obj[r.(string)]; !present {
						fail(out, rule, path, "required key %q is absent", r)
					}
				}
			}
			props, _ := n["properties"].(map[string]any)
			for _, k := range sortedKeys(obj) {
				if sub, ok := props[k]; ok {
					validate(sub, obj[k], path+"/"+k, rule, out)
				} else if add, ok := n["additionalProperties"]; ok {
					validate(add, obj[k], path+"/"+k, rule, out)
				}
			}
		}
		if list, ok := n["allOf"].([]any); ok {
			for _, sub := range list {
				validate(sub, v, path, rule, out)
			}
		}
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isType(v any, t string) bool {
	switch t {
	case "string":
		_, ok := v.(string)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "null":
		return v == nil
	}
	return false
}

func typeName(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case map[string]any:
		return "object"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", v)
}

func inEnum(list []any, v any) bool {
	for _, e := range list {
		if e == v {
			return true
		}
	}
	return false
}
