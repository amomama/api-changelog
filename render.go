package changelog

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	definitionPrefix = "#/definitions/"
	refKey           = "$ref"
	typeKey          = "type"
	formatKey        = "format"
	enumKey          = "enum"
	requiredKey      = "required"
	propertiesKey    = "properties"
	itemsKey         = "items"
	allOfKey         = "allOf"
	enumNamesKey     = "x-enum-varnames"
	defaultKey       = "default"
	descriptionKey   = "description"
	exampleKey       = "example"
	arrayType        = "array"
	objectType       = "object"
	stringType       = "string"
	integerType      = "integer"
	numberType       = "number"
	booleanType      = "boolean"
	sampleString     = "string"
	sampleNumber     = 0
	sampleBoolean    = true
	arrayItems       = "[]"
	bodyIn           = "body"
	jsonIndent       = "  "
	jsonFenceStart   = "```json\n"
	jsonFenceEnd     = "\n```"
	stateAdded       = "added"
	stateChanged     = "changed"
	stateRemoved     = "removed"
	contentSeparator = " · "
	partSeparator    = "\n\n"
	listItemPrefix   = "- "
	noValue          = "none"
)

func renderSection(k endpointKey, baseSpec, headSpec *spec) (string, error) {
	base, err := buildView(baseSpec, k)
	if err != nil {
		return "", err
	}
	head, err := buildView(headSpec, k)
	if err != nil {
		return "", err
	}
	switch {
	case base == nil && head == nil:
		return "", nil
	case base == nil:
		return strings.Join(addedParts(k, head), partSeparator), nil
	case head == nil:
		parts := []string{title(k, stateRemoved), summaryLine(base.op.Summary)}
		return strings.Join(slices.DeleteFunc(parts, isEmpty), partSeparator), nil
	}
	list := changes(base, head)
	if len(list) == 0 {
		return "", nil
	}
	return title(k, stateChanged) + partSeparator + bulletList(list), nil
}

func isEmpty(s string) bool { return s == "" }

func title(k endpointKey, state string) string {
	return fmt.Sprintf("### %s · %s", k, state)
}

func summaryLine(summary string) string {
	if summary == "" {
		return ""
	}
	return "**" + escapeMarkdown(summary) + "**"
}

func escapeMarkdown(s string) string {
	return strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(s)
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func bulletList(items []string) string {
	lines := make([]string, len(items))
	for i, item := range items {
		lines[i] = listItemPrefix + item
	}
	return strings.Join(lines, "\n")
}

func jsonBlock(s string) string {
	return jsonFenceStart + s + jsonFenceEnd
}

func addedParts(k endpointKey, head *view) []string {
	parts := []string{title(k, stateAdded), summaryLine(head.op.Summary)}
	if head.op.Deprecated {
		parts = append(parts, "**Deprecated.**")
	}
	parts = append(parts, escapeMarkdown(head.op.Description), contentTypes(head.op))
	parts = append(parts, parameterList(head.params), requestBody(head.body))
	parts = append(parts, responseBlocks(head)...)
	return slices.DeleteFunc(parts, isEmpty)
}

func contentTypes(op operation) string {
	var halves []string
	if len(op.Consumes) > 0 {
		halves = append(halves, "Consumes "+codeList(op.Consumes))
	}
	if len(op.Produces) > 0 {
		halves = append(halves, "Produces "+codeList(op.Produces))
	}
	return strings.Join(halves, contentSeparator)
}

func codeList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "`" + item + "`"
	}
	return strings.Join(quoted, ", ")
}

func parameterList(params []parameter) string {
	if len(params) == 0 {
		return ""
	}
	lines := make([]string, len(params))
	for i, p := range params {
		lines[i] = fmt.Sprintf("`%s` %s", p.Name, parameterText(p))
	}
	return "**Parameters**" + partSeparator + bulletList(lines)
}

func parameterText(p parameter) string {
	text := p.In
	if typ := parameterType(p); typ != "" {
		text += ", " + facts(typ, enumValues(p.Enum, nil), valueText(p.Default))
	}
	if p.Required {
		text += ", required"
	}
	return withDescription("("+text+")", p.Description)
}

func parameterType(p parameter) string {
	return withFormat(p.Type, p.Format)
}

func requestBody(body *bodyView) string {
	if body == nil {
		return ""
	}
	heading := "**Request body** `" + body.name + "`"
	if body.required {
		heading += " (required)"
	}
	return heading + partSeparator + jsonBlock(body.example)
}

type responseGroup struct {
	name    string
	example string
	codes   []string
}

func groupResponses(resp map[string]respView) []responseGroup {
	var groups []responseGroup
	for _, code := range slices.Sorted(maps.Keys(resp)) {
		r := resp[code]
		i := slices.IndexFunc(groups, func(g responseGroup) bool { return g.name == r.name && g.example == r.example })
		if i < 0 {
			groups = append(groups, responseGroup{r.name, r.example, nil})
			i = len(groups) - 1
		}
		groups[i].codes = append(groups[i].codes, code)
	}
	return groups
}

func responseBlocks(head *view) []string {
	var blocks []string
	for _, g := range groupResponses(head.resp) {
		blocks = append(blocks, responseBlock(g, head))
	}
	return blocks
}

func responseBlock(g responseGroup, head *view) string {
	heading := "**Response** without body"
	if g.name != "" {
		heading = "**Response** `" + g.name + "`"
	}
	lines := make([]string, len(g.codes))
	for i, code := range g.codes {
		lines[i] = fmt.Sprintf("`%s` %s", code, oneLine(escapeMarkdown(head.resp[code].description)))
	}
	block := heading + partSeparator + bulletList(lines)
	if g.name == "" {
		return block
	}
	return block + partSeparator + jsonBlock(g.example)
}

func Render(base, head []byte) (string, error) {
	baseSpec, err := parseSpec(base)
	if err != nil {
		return "", err
	}
	headSpec, err := parseSpec(head)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, k := range endpointKeys(baseSpec, headSpec) {
		section, err := renderSection(k, baseSpec, headSpec)
		if err != nil {
			return "", fmt.Errorf("%s: %w", k, err)
		}
		if section != "" {
			parts = append(parts, section)
		}
	}
	return strings.Join(parts, partSeparator), nil
}
