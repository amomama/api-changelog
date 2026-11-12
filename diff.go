package changelog

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
)

func changes(base, head *view) []string {
	var list []string
	switch {
	case !base.op.Deprecated && head.op.Deprecated:
		list = append(list, "Deprecated")
	case base.op.Deprecated && !head.op.Deprecated:
		list = append(list, "No longer deprecated")
	}
	list = append(list, listChange("Consumes", base.op.Consumes, head.op.Consumes)...)
	list = append(list, listChange("Produces", base.op.Produces, head.op.Produces)...)
	list = append(list, parameterChanges(base.params, head.params)...)
	list = append(list, bodyChanges(base.body, head.body)...)
	return append(list, responseChanges(base.resp, head.resp)...)
}

func listChange(label string, base, head []string) []string {
	if slices.Equal(base, head) {
		return nil
	}
	from, to := cmp.Or(codeList(base), noValue), cmp.Or(codeList(head), noValue)
	return []string{fmt.Sprintf("%s changed from %s to %s", label, from, to)}
}

func parameterChanges(base, head []parameter) []string {
	b, h := paramsByName(base), paramsByName(head)
	var list []string
	for _, name := range unionKeys(b, h) {
		old, inBase := b[name]
		cur, inHead := h[name]
		label := fmt.Sprintf("Parameter `%s`", name)
		switch {
		case !inBase:
			list = append(list, label+" added "+parameterText(cur))
		case !inHead:
			list = append(list, label+" removed")
		default:
			if old.In != cur.In {
				list = append(list, fmt.Sprintf("%s moved from %s to %s", label, old.In, cur.In))
			}
			list = append(list, typeChange(label, parameterType(old), parameterType(cur))...)
			list = append(list, requiredChange(label, old.Required, cur.Required)...)
			list = append(list, enumChanges(label, enumValues(old.Enum, nil), enumValues(cur.Enum, nil))...)
			list = append(list, constraintChanges(label, old.constraints, cur.constraints)...)
		}
	}
	return list
}

func paramsByName(params []parameter) map[string]parameter {
	out := make(map[string]parameter, len(params))
	for _, p := range params {
		out[p.Name] = p
	}
	return out
}

func bodyChanges(base, head *bodyView) []string {
	switch {
	case base == nil && head == nil:
		return nil
	case base == nil:
		return []string{"Request body added"}
	case head == nil:
		return []string{"Request body removed"}
	}
	list := requiredChange("Request body", base.required, head.required)
	for _, c := range fieldChanges(base.fields, head.fields) {
		list = append(list, "Request body: "+c)
	}
	return list
}

func responseChanges(base, head map[string]respView) []string {
	var order []string
	codes := map[string][]string{}
	for _, code := range unionKeys(base, head) {
		for _, c := range responseCodeChanges(base, head, code) {
			if _, ok := codes[c]; !ok {
				order = append(order, c)
			}
			codes[c] = append(codes[c], code)
		}
	}
	list := make([]string, len(order))
	for i, c := range order {
		label := "Response "
		if len(codes[c]) > 1 {
			label = "Responses "
		}
		list[i] = label + codeList(codes[c]) + c
	}
	return list
}

func responseCodeChanges(base, head map[string]respView, code string) []string {
	old, inBase := base[code]
	cur, inHead := head[code]
	switch {
	case !inBase:
		return []string{" added"}
	case !inHead:
		return []string{" removed"}
	case old.example == "" && cur.example != "":
		return []string{" body added"}
	case old.example != "" && cur.example == "":
		return []string{" body removed"}
	}
	list := fieldChanges(old.fields, cur.fields)
	for i, c := range list {
		list[i] = ": " + c
	}
	return list
}

func fieldChanges(base, head map[string]field) []string {
	var list []string
	added, removed := map[string]bool{}, map[string]bool{}
	for _, name := range unionKeys(base, head) {
		old, inBase := base[name]
		cur, inHead := head[name]
		label := fmt.Sprintf("field `%s`", name)
		switch {
		case !inBase:
			added[name] = true
			if !added[cur.parent] {
				text := label + " added (" + facts(cur.typ, cur.enum, cur.def, cur.constraints) + ")"
				list = append(list, withDescription(text, cur.description))
			}
		case !inHead:
			removed[name] = true
			if !removed[old.parent] {
				list = append(list, label+" removed")
			}
		default:
			list = append(list, typeChange(label, old.typ, cur.typ)...)
			list = append(list, requiredChange(label, old.required, cur.required)...)
			list = append(list, enumChanges(label, old.enum, cur.enum)...)
			list = append(list, constraintChanges(label, old.constraints, cur.constraints)...)
		}
	}
	return list
}

func typeChange(label, base, head string) []string {
	if base == head {
		return nil
	}
	return []string{fmt.Sprintf("%s type changed from `%s` to `%s`", label, base, head)}
}

func requiredChange(label string, base, head bool) []string {
	switch {
	case !base && head:
		return []string{label + " is now required"}
	case base && !head:
		return []string{label + " is no longer required"}
	}
	return nil
}

func enumChanges(label string, base, head []enumValue) []string {
	var list []string
	for _, v := range base {
		if !hasValue(head, v.value) {
			list = append(list, fmt.Sprintf("%s value %s removed", label, v.quoted()))
		}
	}
	for _, v := range head {
		if !hasValue(base, v.value) {
			list = append(list, fmt.Sprintf("%s value %s added", label, v.quoted()))
		}
	}
	return list
}

func constraintChanges(label string, base, head constraints) []string {
	var list []string
	for _, key := range constraintKeys {
		old, inBase := base[key]
		cur, inHead := head[key]
		switch {
		case !inBase && inHead:
			list = append(list, fmt.Sprintf("%s %s `%s` added", label, key, cur))
		case inBase && !inHead:
			list = append(list, fmt.Sprintf("%s %s `%s` removed", label, key, old))
		case old != cur:
			list = append(list, fmt.Sprintf("%s %s changed from `%s` to `%s`", label, key, old, cur))
		}
	}
	return list
}

func hasValue(enum []enumValue, value string) bool {
	return slices.ContainsFunc(enum, func(e enumValue) bool { return e.value == value })
}

func unionKeys[V any](a, b map[string]V) []string {
	keys := slices.AppendSeq(slices.Collect(maps.Keys(a)), maps.Keys(b))
	slices.Sort(keys)
	return slices.Compact(keys)
}
