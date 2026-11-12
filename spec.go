package changelog

import (
	"bytes"
	"cmp"
	"encoding/json"
	"maps"
	"slices"
	"strings"
)

type operation struct {
	Summary     string              `json:"summary"`
	Description string              `json:"description"`
	Deprecated  bool                `json:"deprecated"`
	Consumes    []string            `json:"consumes"`
	Produces    []string            `json:"produces"`
	Parameters  []parameter         `json:"parameters"`
	Responses   map[string]response `json:"responses"`
}

type parameter struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Type        string          `json:"type"`
	Format      string          `json:"format"`
	Enum        []any           `json:"enum"`
	Default     any             `json:"default"`
	Required    bool            `json:"required"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

type response struct {
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

type spec struct {
	Paths       map[string]map[string]json.RawMessage `json:"paths"`
	Definitions map[string]json.RawMessage            `json:"definitions"`
}

type endpointKey struct{ path, method string }

func (k endpointKey) String() string {
	return strings.ToUpper(k.method) + " " + k.path
}

func parseSpec(data []byte) (*spec, error) {
	var sp spec
	return &sp, json.Unmarshal(data, &sp)
}

func endpointKeys(a, b *spec) []endpointKey {
	seen := map[endpointKey]bool{}
	for _, sp := range []*spec{a, b} {
		for path, ops := range sp.Paths {
			for method := range ops {
				seen[endpointKey{path, method}] = true
			}
		}
	}
	return slices.SortedFunc(maps.Keys(seen), func(a, b endpointKey) int {
		return cmp.Or(strings.Compare(a.path, b.path), strings.Compare(a.method, b.method))
	})
}

type member struct {
	key   string
	value any
}

type omap []member

func (m omap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, e := range m {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := marshal(e.key)
		if err != nil {
			return nil, err
		}
		value, err := marshal(e.value)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func (m omap) get(key string) any {
	for _, e := range m {
		if e.key == key {
			return e.value
		}
	}
	return nil
}

func (m omap) ref() (string, bool) {
	s, ok := m.get(refKey).(string)
	if !ok {
		return "", false
	}
	return strings.CutPrefix(s, definitionPrefix)
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

func parseOrdered(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return readValue(dec)
}

func readValue(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	if delim == '[' {
		items := []any{}
		for dec.More() {
			item, err := readValue(dec)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		_, err := dec.Token()
		return items, err
	}
	members := omap{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		value, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		members = append(members, member{key.(string), value}) //nolint:forcetypeassert // object keys are strings
	}
	_, err = dec.Token()
	return members, err
}

type inliner struct{ defs map[string]json.RawMessage }

func (in inliner) inline(v any, path []string) (any, error) {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			inlined, err := in.inline(item, path)
			if err != nil {
				return nil, err
			}
			out[i] = inlined
		}
		return out, nil
	case omap:
		return in.inlineObject(t, path)
	}
	return v, nil
}

func (in inliner) inlineObject(obj omap, path []string) (omap, error) {
	name, ok := obj.ref()
	raw, found := in.defs[name]
	if !ok || !found || slices.Contains(path, name) {
		return in.inlineMembers(obj, path)
	}
	parsed, err := parseOrdered(raw)
	if err != nil {
		return nil, err
	}
	def, ok := parsed.(omap)
	if !ok {
		return in.inlineMembers(obj, path)
	}
	out, err := in.inlineObject(def, append(slices.Clone(path), name))
	if err != nil {
		return nil, err
	}
	siblings, err := in.inlineMembers(withoutKey(obj, refKey), path)
	if err != nil {
		return nil, err
	}
	return overlay(out, siblings), nil
}

func (in inliner) inlineMembers(obj omap, path []string) (omap, error) {
	out := make(omap, len(obj))
	for i, e := range obj {
		value, err := in.inline(e.value, path)
		if err != nil {
			return nil, err
		}
		out[i] = member{e.key, value}
	}
	return out, nil
}

func withoutKey(obj omap, key string) omap {
	return slices.DeleteFunc(slices.Clone(obj), func(e member) bool { return e.key == key })
}

func overlay(base, top omap) omap {
	out := slices.Clone(base)
	for _, e := range top {
		i := slices.IndexFunc(out, func(m member) bool { return m.key == e.key })
		if i < 0 {
			out = append(out, e)
		} else {
			out[i] = e
		}
	}
	return out
}
