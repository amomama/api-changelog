package changelog_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	changelog "github.com/amomama/api-changelog"
)

func swagger(paths, definitions string) []byte {
	return []byte(fmt.Sprintf(`{"paths":%s,"definitions":%s}`, paths, definitions))
}

type renderCase struct {
	name string
	base []byte
	head []byte
	want string
}

var renderCases = []renderCase{
	{
		name: "endpoint added",
		base: swagger(`{}`, `{}`),
		head: swagger(`{"/pets":{"get":{"summary":"List pets","produces":["application/json"],
				"parameters":[{"name":"limit","in":"query","type":"integer","required":true}],
				"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"name":{"type":"string"}}}}`),
		want: "### GET /pets · added\n\n**List pets**\n\nProduces `application/json`\n\n" +
			"**Parameters**\n\n- `limit` (query, integer, required)\n\n" +
			"**Response** `Pet`\n\n- `200` OK\n\n```json\n{\n  \"name\": \"string\"\n}\n```",
	},
	{
		name: "endpoint removed",
		base: swagger(`{"/pets":{"get":{"summary":"List pets","responses":{"200":{"description":"OK"}}}}}`, `{}`),
		head: swagger(`{}`, `{}`),
		want: "### GET /pets · removed\n\n**List pets**",
	},
	{
		name: "endpoint changed",
		base: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK"}}}}}`, `{}`),
		head: swagger(`{"/pets":{"get":{"deprecated":true,"produces":["application/json"],
				"responses":{"200":{"description":"OK"},"404":{"description":"Not found"}}}}}`, `{}`),
		want: "### GET /pets · changed\n\n- Deprecated\n- Produces changed from none to `application/json`\n" +
			"- Response `404` added",
	},
	{
		name: "no changes",
		base: swagger(`{"/pets":{"get":{"summary":"List","responses":{"200":{"description":"OK"}}}}}`, `{}`),
		head: swagger(`{"/pets":{"get":{"summary":"List","responses":{"200":{"description":"OK"}}}}}`, `{}`),
		want: "",
	},
	{
		name: "nested field added lists only the parent",
		base: swagger(`{"/pets":{"post":{"parameters":[{"name":"body","in":"body","schema":{"$ref":"#/definitions/Pet"}}],
				"responses":{}}}}`, `{"Pet":{"type":"object","properties":{"name":{"type":"string"}}}}`),
		head: swagger(`{"/pets":{"post":{"parameters":[{"name":"body","in":"body","schema":{"$ref":"#/definitions/Pet"}}],
				"responses":{}}}}`, `{"Pet":{"type":"object","properties":{"name":{"type":"string"},
				"owner":{"type":"object","properties":{"id":{"type":"integer"}}}}}}`),
		want: "### POST /pets · changed\n\n- Request body: field `owner` added (object)",
	},
	{
		name: "nested field removed lists only the parent",
		base: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"name":{"type":"string"},
				"owner":{"type":"object","properties":{"id":{"type":"integer"}}}}}}`),
		head: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"name":{"type":"string"}}}}`),
		want: "### GET /pets · changed\n\n- Response `200`: field `owner` removed",
	},
	{
		name: "nested child added to existing parent",
		base: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"owner":{"type":"object","properties":{"id":{"type":"integer"}}}}}}`),
		head: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"owner":{"type":"object","properties":{"id":{"type":"integer"},
				"name":{"type":"string"}}}}}}`),
		want: "### GET /pets · changed\n\n- Response `200`: field `owner.name` added (string)",
	},
	{
		name: "enum value added and removed",
		base: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"status":{"type":"string","enum":["new","old"]}}}}`),
		head: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"status":{"type":"string","enum":["new","sold"]}}}}`),
		want: "### GET /pets · changed\n\n- Response `200`: field `status` value `old` removed\n" +
			"- Response `200`: field `status` value `sold` added",
	},
	{
		name: "parameter required changed",
		base: swagger(`{"/pets":{"get":{"parameters":[{"name":"limit","in":"query","type":"integer"}],
				"responses":{}}}}`, `{}`),
		head: swagger(`{"/pets":{"get":{"parameters":[{"name":"limit","in":"query","type":"integer","required":true}],
				"responses":{}}}}`, `{}`),
		want: "### GET /pets · changed\n\n- Parameter `limit` is now required",
	},
	{
		name: "definition rename yields no changes",
		base: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK","schema":{"$ref":"#/definitions/Pet"}}}}}}`,
			`{"Pet":{"type":"object","properties":{"name":{"type":"string"}}}}`),
		head: swagger(`{"/pets":{"get":{"responses":{"200":{"description":"OK",
				"schema":{"$ref":"#/definitions/Animal"}}}}}}`,
			`{"Animal":{"type":"object","properties":{"name":{"type":"string"}}}}`),
		want: "",
	},
}

func TestRender(t *testing.T) {
	for _, tt := range renderCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changelog.Render(tt.base, tt.head)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
