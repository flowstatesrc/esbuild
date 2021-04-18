package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImportQuery(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "import {query} from \"./query\";\nfs.executeQuery(query);\n",
		"/query.js": "import filter from \"./filter\";\nexport const query = sql`select * from foo where ${filter}`;\n",
		"/filter.js": "const cond = sql.p`foo = ${window.bar}`;\nexport default cond;\n",
	}, nil, "/app.js")

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select * from foo where foo = $1",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "query.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 2.0,
				"fileName": "app.js",
			},
		},
		"id": "ZMgpomtDsreEoz4spXNvf-xsrLEKUHsYvzOBYUNH",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var cond = void 0;`) // was inlined into the query
	assert.Contains(t, code, `var query = {query: "ZMgpomtDsreEoz4spXNvf-xsrLEKUHsYvzOBYUNH", text: "select * from foo where foo = $1", params: {$1: window.bar}};`)
	assert.Contains(t, code, `fs.executeQuery(query);`)
}

func TestImportNamespace(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "import * as queries from \"./query\";\nfs.executeQuery(queries.query);\n",
		"/query.js": "export const query = sql`insert into foo (text) values (%{bar})`;\n",
	}, nil, "/app.js")

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "insert into foo (text) values ($1)",
		"type": "insert",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 1.0,
			"fileName": "query.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 2.0,
				"fileName": "app.js",
			},
		},
		"id": "xg4Am1Hr3jMdrxVjdeB6QpPPXlM187k4yBjCeM8S",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var query = {query: "xg4Am1Hr3jMdrxVjdeB6QpPPXlM187k4yBjCeM8S", text: "insert into foo (text) values ($1)", params: {bar: "__PARAM_"}};`)
	assert.Contains(t, code, `fs.executeQuery(query);`)
}

func TestImportReexportedName(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "import {aliased} from \"./rexported\";\nfs.executeQuery(aliased);\n",
		"/query.js": "export const query = sql`ALTER TABLE distributors RENAME COLUMN address TO city`;\n",
		"/rexported.js": "import {query} from \"./star\";\nexport const aliased = query;\n",
		"/star.js": "export * from \"./query\";\n",
	}, nil, "/app.js")

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "ALTER TABLE distributors RENAME COLUMN address TO city",
		"type": "other",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 1.0,
			"fileName": "query.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 2.0,
				"fileName": "app.js",
			},
		},
		"id": "dk-72m8Ns4q6aORxeXN9N5TQRwqHSZnorIFQvtiY",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var query = {query: "dk-72m8Ns4q6aORxeXN9N5TQRwqHSZnorIFQvtiY", text: "ALTER TABLE distributors RENAME COLUMN address TO city", params: {}};`)
	assert.Contains(t, code, `fs.executeQuery(aliased);`)
}

func TestImportAliasedName(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "import {query as aliased} from \"./query\";\nfs.executeQuery(aliased);\n",
		"/query.js": "export const query = sql`delete from foo where bar = %{bar} and baz = %{baz}`;\n",
	}, nil, "/app.js")

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "delete from foo where bar = $1 and baz = $2",
		"type": "delete",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 1.0,
			"fileName": "query.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 2.0,
				"fileName": "app.js",
			},
		},
		"id": "A6in6tB2ANhLSehRXnz7yTPVMkjSh1hgfQtGSxlm",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var query = {query: "A6in6tB2ANhLSehRXnz7yTPVMkjSh1hgfQtGSxlm", text: "delete from foo where bar = $1 and baz = $2", params: {bar: "__PARAM_", baz: "__PARAM_"}};`)
	assert.Contains(t, code, `fs.executeQuery(query);`)
}