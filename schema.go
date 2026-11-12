package changelog

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type schema struct {
	name    string
	example string
	fields  map[string]field
}

type field struct {
	parent      string
	typ         string
	required    bool
	enum        []enumValue
	def         string
	constraints constraints
	description string
}

type constraints map[string]string

func constraintsOf(node omap) constraints {
	out := constraints{}
	for _, key := range constraintKeys {
		if value := node.get(key); value != nil {
			out[key] = valueText(value)
		}
	}
	return out
}

type enumValue struct{ value, name string }

func (e enumValue) String() string {
	return strings.TrimSpace(e.value + " " + e.name)
}

func (e enumValue) quoted() string {
	if e.name == "" {
		return "`" + e.value + "`"
	}
	return "`" + e.value + "` (" + e.name + ")"
}

func (in inliner) parseSchema(raw json.RawMessage) (schema, error) {
	name, err := schemaName(raw)
	if err != nil {
		return schema{}, err
	}
	parsed, err := parseOrdered(raw)
	if err != nil {
		return schema{}, err
	}
	inlined, err := in.inline(parsed, nil)
	if err != nil {
		return schema{}, err
	}
	compact, err := marshal(sample(inlined))
	if err != nil {
		return schema{}, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, compact, "", jsonIndent); err != nil {
		return schema{}, err
	}
	fields := map[string]field{}
	if root, ok := inlined.(omap); ok {
		collectFields(root, "", fields)
	}
	return schema{name, out.String(), fields}, nil
}

func sample(v any) any {
	node, _ := v.(omap)
	node = mergeAllOf(node)
	for _, key := range []string{exampleKey, defaultKey} {
		if value := node.get(key); value != nil {
			return value
		}
	}
	if enum, _ := node.get(enumKey).([]any); len(enum) > 0 {
		return enum[0]
	}
	switch typ, _ := node.get(typeKey).(string); typ {
	case arrayType:
		return []any{sample(node.get(itemsKey))}
	case stringType:
		return sampleString
	case integerType, numberType:
		return sampleNumber
	case booleanType:
		return sampleBoolean
	}
	props, _ := node.get(propertiesKey).(omap)
	out := make(omap, len(props))
	for i, p := range props {
		out[i] = member{p.key, sample(p.value)}
	}
	return out
}

func schemaName(raw json.RawMessage) (string, error) {
	var s struct {
		Ref   string `json:"$ref"`
		Type  string `json:"type"`
		Items struct {
			Ref string `json:"$ref"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	switch {
	case s.Ref != "":
		return strings.TrimPrefix(s.Ref, definitionPrefix), nil
	case s.Type == arrayType && s.Items.Ref != "":
		return arrayType + "<" + strings.TrimPrefix(s.Items.Ref, definitionPrefix) + ">", nil
	}
	return s.Type, nil
}

func mergeAllOf(node omap) omap {
	members, ok := node.get(allOfKey).([]any)
	if !ok {
		return node
	}
	var out omap
	for _, m := range members {
		if part, ok := m.(omap); ok {
			out = overlay(out, mergeAllOf(part))
		}
	}
	return overlay(out, withoutKey(node, allOfKey))
}

func collectFields(node omap, name string, out map[string]field) {
	node = mergeAllOf(node)
	if node.get(typeKey) != arrayType {
		addProperties(node, name, name, out)
		return
	}
	if items, ok := node.get(itemsKey).(omap); ok {
		addProperties(mergeAllOf(items), name+arrayItems, name, out)
	}
}

func addProperties(node omap, prefix, parent string, out map[string]field) {
	props, _ := node.get(propertiesKey).(omap)
	required := stringValues(node.get(requiredKey))
	for _, p := range props {
		child, _ := p.value.(omap)
		child = mergeAllOf(child)
		name := p.key
		if prefix != "" {
			name = prefix + "." + p.key
		}
		description, _ := child.get(descriptionKey).(string)
		out[name] = field{
			parent:      parent,
			typ:         typeLabel(child),
			required:    slices.Contains(required, p.key),
			enum:        enumValues(child.get(enumKey), child.get(enumNamesKey)),
			def:         valueText(child.get(defaultKey)),
			constraints: constraintsOf(child),
			description: description,
		}
		collectFields(child, name, out)
	}
}

func typeLabel(node omap) string {
	node = mergeAllOf(node)
	if name, ok := node.ref(); ok {
		return name
	}
	typ, _ := node.get(typeKey).(string)
	if typ == arrayType {
		items, _ := node.get(itemsKey).(omap)
		return arrayType + "<" + typeLabel(items) + ">"
	}
	format, _ := node.get(formatKey).(string)
	return withFormat(cmp.Or(typ, objectType), format)
}

func withFormat(typ, format string) string {
	if format == "" {
		return typ
	}
	return typ + " " + format
}

func stringValues(v any) []string {
	items, _ := v.([]any)
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = fmt.Sprint(item)
	}
	return out
}

func valueText(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func enumValues(values, names any) []enumValue {
	vs, ns := stringValues(values), stringValues(names)
	out := make([]enumValue, len(vs))
	for i, v := range vs {
		out[i].value = v
		if i < len(ns) {
			out[i].name = ns[i]
		}
	}
	return out
}

func facts(typ string, enum []enumValue, def string, limits constraints) string {
	parts := []string{typ}
	if len(enum) > 0 {
		values := make([]string, len(enum))
		for i, e := range enum {
			values[i] = e.String()
		}
		parts = append(parts, "one of "+strings.Join(values, ", "))
	}
	if def != "" {
		parts = append(parts, "default "+def)
	}
	for _, key := range constraintKeys {
		if value, ok := limits[key]; ok {
			parts = append(parts, key+" "+value)
		}
	}
	return strings.Join(parts, ", ")
}

func withDescription(text, description string) string {
	if description == "" {
		return text
	}
	return text + ": " + oneLine(escapeMarkdown(description))
}

type bodyView struct {
	schema
	required bool
}

type respView struct {
	description string
	schema
}

type view struct {
	op     operation
	params []parameter
	body   *bodyView
	resp   map[string]respView
}

func mergeBodies(bodies []parameter) (parameter, error) {
	props := make(omap, len(bodies))
	required := []any{}
	merged := parameter{In: bodyIn}
	for i, p := range bodies {
		parsed, err := parseOrdered(p.Schema)
		if err != nil {
			return parameter{}, err
		}
		schema, _ := parsed.(omap)
		if p.Description != "" {
			schema = overlay(schema, omap{{descriptionKey, p.Description}})
		}
		props[i] = member{p.Name, schema}
		if p.Required {
			required = append(required, p.Name)
			merged.Required = true
		}
	}
	raw, err := marshal(omap{{typeKey, objectType}, {propertiesKey, props}, {requiredKey, required}})
	merged.Schema = raw
	return merged, err
}

func buildView(sp *spec, k endpointKey) (*view, error) {
	raw, ok := sp.Paths[k.path][k.method]
	if !ok {
		return nil, nil //nolint:nilnil // a missing endpoint is not an error
	}
	v := &view{resp: map[string]respView{}}
	if err := json.Unmarshal(raw, &v.op); err != nil {
		return nil, err
	}
	in := inliner{sp.Definitions}
	var bodies []parameter
	for _, p := range v.op.Parameters {
		if p.In == bodyIn {
			bodies = append(bodies, p)
		} else {
			v.params = append(v.params, p)
		}
	}
	if len(bodies) > 1 {
		merged, err := mergeBodies(bodies)
		if err != nil {
			return nil, err
		}
		bodies = []parameter{merged}
	}
	for _, p := range bodies {
		s, err := in.parseSchema(p.Schema)
		if err != nil {
			return nil, err
		}
		v.body = &bodyView{s, p.Required}
	}
	for code, r := range v.op.Responses {
		rv := respView{description: r.Description}
		if len(r.Schema) > 0 {
			s, err := in.parseSchema(r.Schema)
			if err != nil {
				return nil, err
			}
			rv.schema = s
		}
		v.resp[code] = rv
	}
	return v, nil
}

var constraintKeys = []string{
	minimumKey, exclusiveMinimumKey, maximumKey, exclusiveMaximumKey, multipleOfKey, minLengthKey, maxLengthKey,
	patternKey, minItemsKey, maxItemsKey, uniqueItemsKey, minPropertiesKey, maxPropertiesKey,
}
