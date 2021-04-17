package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/evanw/esbuild/pkg/api"
)

func TestInlineQuery(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "fs.executeQuery(sql`select 1`);\n",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select 1",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 1.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 1.0,
				"fileName": "app.js",
			},
		},
		"id": "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `fs.executeQuery({query: "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL", text: "select 1", params: {}})`)
}

func TestInlineQueryWithFragment(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "const table = sql.p`foo`;\nconst fallback = sql.p`fallback`;\nfs.executeQuery(sql`select 1 from ${table || fallback}`);\n",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select 1 from ${fragment1}",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 3.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 3.0,
				"fileName": "app.js",
			},
		},
		"id": "HoVHYay37wWiR5AQ7lDnUF3FiGZAVW0VxayxAfth",
		"fragments": []interface{}{
			[]interface{}{
				map[string]interface{}{
					"id": "LCa0a2j_xo_5m0U8HTBBNBNCLXBkg7-g-YpeiGJm",
					"query":    "foo",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     1.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id": "XH7iB0tlhT9x_FoBzhlP8m3u322qzbcVxr7v39Pz",
					"query":    "fallback",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     2.0,
						"fileName": "app.js",
					},
				},
			},
		},
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var table = {query: "LCa0a2j_xo_5m0U8HTBBNBNCLXBkg7-g-YpeiGJm", text: "foo", params: {}};`)
	assert.Contains(t, code, `fallback = {query: "XH7iB0tlhT9x_FoBzhlP8m3u322qzbcVxr7v39Pz", text: "fallback", params: {}};`)
	assert.Contains(t, code, `sql.merge({query: "HoVHYay37wWiR5AQ7lDnUF3FiGZAVW0VxayxAfth", text: "select 1 from ${fragment1}", params: {}}, table || fallback));`)
}

func TestQueryVar(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "let bar = 12, baz = 'foo';\nconst query = sql`select * from foo where bar = ${bar} and baz = ${baz}`;\nfs.executeQuery(query);\n",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select * from foo where bar = $1 and baz = $2",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 3.0,
				"fileName": "app.js",
			},
		},
		"id": "1FfqlKV9DHWV-e2cGUKAAXu6cILqFYOegLBlAT5o",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var bar = 12;`)
	assert.Contains(t, code, `var baz = "foo";`)
	assert.Contains(t, code, `var query = {query: "1FfqlKV9DHWV-e2cGUKAAXu6cILqFYOegLBlAT5o", text: "select * from foo where bar = $1 and baz = $2", params: {$1: bar, $2: baz}};`)
	assert.Contains(t, code, `fs.executeQuery(query);`)
}

func TestQueryUsedTwice(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "import \"./other\";\nimport {query} from \"./query\";\nfs.executeQuery(query);\n",
		"/other.js": "import {query} from \"./query\";\nfs.executeQuery(query);\n",
		"/query.js": "let bar = 12, baz = 'foo';\nexport const query = sql`select * from foo where bar = ${bar} and baz = ${baz}`;\n",
	}, &api.FlowStateOptions{}, "/app.js")

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select * from foo where bar = $1 and baz = $2",
		"type": "select",
		"isPublic": true,
		"clientReferences": 2.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "query.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 3.0,
				"fileName": "app.js",
			},
			map[string]interface{}{
				"line": 2.0,
				"fileName": "other.js",
			},
		},
		"id": "1FfqlKV9DHWV-e2cGUKAAXu6cILqFYOegLBlAT5o",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var bar = 12;`)
	assert.Contains(t, code, `var baz = "foo";`)
	assert.Contains(t, code, `var query = {query: "1FfqlKV9DHWV-e2cGUKAAXu6cILqFYOegLBlAT5o", text: "select * from foo where bar = $1 and baz = $2", params: {$1: bar, $2: baz}};`)
	assert.Contains(t, code, `fs.executeQuery(query);`)
}

func TestQueriesObjectLiteral(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "const queries = {\n  'query': sql`SELECT * FROM object_literal WHERE ${query}`,\n  'prop': sql`SELECT * FROM object_property`\n};\nfunction dynamic(key) {\n  return fs.executeQuery(queries[key]);\n}\nfs.executeQuery(queries.prop);\ndynamic(window.location);",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "SELECT * FROM object_literal WHERE $1",
		"type": "select",
		"isPublic": true,
		"clientReferences": 2.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 6.0,
				"fileName": "app.js",
			},
			map[string]interface{}{
				"line": 8.0,
				"fileName": "app.js",
			},
		},
		"id": "1KeaRXO1OnvO5WtC74BPxS5w_XhnuwLiSvop12z1",
	}, whitelist[0])
	assert.Equal(t, map[string]interface{}{
		"query": "SELECT * FROM object_property",
		"type": "select",
		"isPublic": true,
		"clientReferences": 2.0,
		"definedAt": map[string]interface{}{
			"line": 3.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 6.0,
				"fileName": "app.js",
			},
			map[string]interface{}{
				"line": 8.0,
				"fileName": "app.js",
			},
		},
		"id": "Rf2BEiMSz4YmBxi8OsmwPjKfqw5Fn94cz383TB7G",
	}, whitelist[1])
	assert.Len(t, whitelist, 2)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `query: {query: "1KeaRXO1OnvO5WtC74BPxS5w_XhnuwLiSvop12z1", text: "SELECT * FROM object_literal WHERE $1", params: {$1: query}},`)
	assert.Contains(t, code, `prop: {query: "Rf2BEiMSz4YmBxi8OsmwPjKfqw5Fn94cz383TB7G", text: "SELECT * FROM object_property", params: {}}`)
	assert.Contains(t, code, `return fs.executeQuery(queries[key]);`)
	assert.Contains(t, code, `fs.executeQuery(queries.prop);`)
}

func TestQueryAliasingAssignments(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "let bar = 12, baz = 'foo';\nlet assignment;\nconst query = sql`select * from foo where bar = ${bar} and baz = ${baz}`;\nconst query2 = query;\nassignment=query2;\nfs.executeQuery(assignment);\n",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select * from foo where bar = $1 and baz = $2",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 3.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 6.0,
				"fileName": "app.js",
			},
		},
		"id": "1FfqlKV9DHWV-e2cGUKAAXu6cILqFYOegLBlAT5o",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var bar = 12;`)
	assert.Contains(t, code, `var baz = "foo";`)
	assert.Contains(t, code, `var query = {query: "1FfqlKV9DHWV-e2cGUKAAXu6cILqFYOegLBlAT5o", text: "select * from foo where bar = $1 and baz = $2", params: {$1: bar, $2: baz}};`)
	assert.Contains(t, code, `fs.executeQuery(assignment);`)
}

func TestPrivateQuery(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "let filter = (window.location) ? sql.p`user_id = %{SESSION.user_id}` : sql.p`%{SESSION.roles}::jsonb ? role`;\nconst query = sql`update foo set bar = ${12} where ${filter}`;\nfs.executeQuery(query);\n",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "update foo set bar = $1 where ${fragment1}",
		"type": "update",
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 3.0,
				"fileName": "app.js",
			},
		},
		"id": "7NpJOlclOBksJP_OqcNhxpmJJJLv7xzyOB7U3AO1",
		"fragments": []interface{}{
			[]interface{}{
				map[string]interface{}{
					"id": "mo_YXFbQvk1YV1_MdJJk2fcJziqIydnlk0-EEwdc",
					"query":    "user_id = ${SESSION.user_id}",
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     1.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id": "KmyrotC7fWU_BAuR_cM5nOT-uYxz1k-u8LZXCa10",
					"query":    "${SESSION.roles}::jsonb ? role",
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     1.0,
						"fileName": "app.js",
					},
				},
			},
		},
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var filter = window.location ? {query: "mo_YXFbQvk1YV1_MdJJk2fcJziqIydnlk0-EEwdc", text: "user_id = ${SESSION.user_id}", params: {}} : {query: "KmyrotC7fWU_BAuR_cM5nOT-uYxz1k-u8LZXCa10", text: "${SESSION.roles}::jsonb ? role", params: {}};`)
	assert.Contains(t, code, `sql.merge({query: "7NpJOlclOBksJP_OqcNhxpmJJJLv7xzyOB7U3AO1", text: "update foo set bar = $1 where ${fragment1}", params: {$1: 12}}, filter);`)
	assert.Contains(t, code, `fs.executeQuery(query);`)
}

func TestNotPrivateQuery(t *testing.T) {
	// If one fragment is public, and another is private then the query is public.
	result := build(map[string]string{
		"/app.js": "let filter = (window.location) ? sql.p`user_id = %{SESSION.user_id}` : sql.p`1 = 1`;\nconst query = sql`update foo set bar = ${12} where ${filter}`;\nfs.executeQuery(query);\n",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "update foo set bar = $1 where ${fragment1}",
		"type": "update",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 3.0,
				"fileName": "app.js",
			},
		},
		"id": "TpZkx2L5A-YVHmYJJRi8u-ejjAeXOLuNq1Gw3TXv",
		"fragments": []interface{}{
			[]interface{}{
				map[string]interface{}{
					"id": "mo_YXFbQvk1YV1_MdJJk2fcJziqIydnlk0-EEwdc",
					"query":    "user_id = ${SESSION.user_id}",
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     1.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id": "JFAYk5QGdzmjTvsNnguUqMZJNEmliJM9kwSI2KRO",
					"query":    "1 = 1",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     1.0,
						"fileName": "app.js",
					},
				},
			},
		},
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `var filter = window.location ? {query: "mo_YXFbQvk1YV1_MdJJk2fcJziqIydnlk0-EEwdc", text: "user_id = ${SESSION.user_id}", params: {}} : {query: "JFAYk5QGdzmjTvsNnguUqMZJNEmliJM9kwSI2KRO", text: "1 = 1", params: {}};`)
	assert.Contains(t, code, `sql.merge({query: "TpZkx2L5A-YVHmYJJRi8u-ejjAeXOLuNq1Gw3TXv", text: "update foo set bar = $1 where ${fragment1}", params: {$1: 12}}, filter);`)
	assert.Contains(t, code, `fs.executeQuery(query);`)
}

func TestMixedQuery(t *testing.T) {
	// A query used on server and client is included in the whitelist

	result := build(map[string]string{
		"/app.js": `

			const query = sql` + "`select 1`" + `;

			export async function server(ctx, a, b, c) {
				ctx.executeQuery(query);
				return a*b + c;
			}

			window.doStuff = async function(a, b) {
				const result = await server(window.fs.beginTx(), a, b, 3);
			};

			fs.executeQuery(query);
			`,
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select 1",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"serverReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 3.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 6.0,
				"fileName": "app.js",
			},
			map[string]interface{}{
				"line": 14.0,
				"fileName": "app.js",
			},
		},
		"id": "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	serverWhitelist := getServerWhitelist(&result)
	assert.Empty(t, serverWhitelist)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, client, `{query: "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL", text: "select 1", params: {}}`)
	assert.Contains(t, client, `fs.executeQuery(query)`)
	assert.NotContains(t, client, `server(`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `{query: "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL", text: "select 1", params: {}}`)
	assert.Contains(t, server, `ctx.executeQuery(query)`)
	assert.Contains(t, server, `server(`)
}

func TestNoServerOnlyQuery(t *testing.T) {
	// Server only queries are not included in the whitelist

	result := build(map[string]string{
		"/app.js": `
			const query = sql` + "`select 1`" + `;

			export async function server(ctx, a, b, c) {
				ctx.executeQuery(query);
				return a*b + c;
			}

			window.doStuff = async function(a, b) {
				const result = await server(window.fs.beginTx(), a, b, 3);
			};
			`,
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.Empty(t, whitelist)

	serverWhitelist := getServerWhitelist(&result)
	assert.NotEmpty(t, serverWhitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select 1",
		"type": "select",
		"isPublic": true,
		"serverReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 5.0,
				"fileName": "app.js",
			},
		},
		"id": "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL",
	}, serverWhitelist[0])
	assert.Len(t, serverWhitelist, 1)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.NotContains(t, client, "select 1")
	assert.NotContains(t, client, "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL")
	assert.NotContains(t, client, `executeQuery(query)`)
	assert.NotContains(t, client, `server(`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `query = {query: "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL", text: "select 1", params: {}}`)
	assert.Contains(t, server, `ctx.executeQuery(query)`)
	assert.Contains(t, server, `server(`)
}

func TestMixedInlineServerOnlyFragment(t *testing.T) {
	// A fragment used directly on the client, but inlined on the server
	// Must be included in the output for both server and client.

	result := build(map[string]string{
		"/app.js": `
			const fragment = sql.p` + "`select id from users`" + `;

			export async function server(ctx, a, b, c) {
				ctx.executeQuery(sql` + "`select * from orders where user_id IN (${fragment})`" + `);
				return a*b + c;
			}

			window.doStuff = async function(a, b) {
				const result = await server(window.fs.beginTx(), a, b, 3);
			};

			fs.executeQuery(sql` + "`${fragment}`);",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select id from users",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 13.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 13.0,
				"fileName": "app.js",
			},
		},
		"id": "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	serverWhitelist := getServerWhitelist(&result)
	assert.NotEmpty(t, serverWhitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select * from orders where user_id IN (select id from users)",
		"type": "select",
		"isPublic": true,
		"serverReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 5.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 5.0,
				"fileName": "app.js",
			},
		},
		"id": "pX6rtyrDUu2zGInsy0CSWuBNOko9LUWbMMfafE-d",
	}, serverWhitelist[0])
	assert.Len(t, serverWhitelist, 1)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.NotContains(t, client, "select * from orders")
	assert.Contains(t, client, "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq")
	assert.NotContains(t, client, "pX6rtyrDUu2zGInsy0CSWuBNOko9LUWbMMfafE-d")
	assert.NotContains(t, client, `server(`)
	assert.Contains(t, client, `fs.executeQuery({query: "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq", text: "select id from users", params: {}})`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.NotContains(t, server, "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq")
	assert.Contains(t, server, `ctx.executeQuery({query: "pX6rtyrDUu2zGInsy0CSWuBNOko9LUWbMMfafE-d", text: "select * from orders where user_id IN (select id from users)", params: {}})`)
	assert.Contains(t, server, `server(`)
}

func TestMixedInlineClientOnlyFragment(t *testing.T) {
	// A fragment used directly on the client, but inlined on the server
	// Must be included in the output for both server and client.

	result := build(map[string]string{
		"/app.js": `
			const fragment = sql.p` + "`select id from users`" + `;

			export async function server(ctx, a, b, c) {
				ctx.executeQuery(sql` + "`${fragment}`" + `);
				return a*b + c;
			}

			window.doStuff = async function(a, b) {
				const result = await server(window.fs.beginTx(), a, b, 3);
			};

			fs.executeQuery(sql` + "`select * from orders where user_id IN (${fragment})`" + `);
			`,
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select * from orders where user_id IN (select id from users)",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 13.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 13.0,
				"fileName": "app.js",
			},
		},
		"id": "pX6rtyrDUu2zGInsy0CSWuBNOko9LUWbMMfafE-d",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	serverWhitelist := getServerWhitelist(&result)
	assert.NotEmpty(t, serverWhitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select id from users",
		"type": "select",
		"isPublic": true,
		"serverReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 5.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 5.0,
				"fileName": "app.js",
			},
		},
		"id": "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq",
	}, serverWhitelist[0])
	assert.Len(t, serverWhitelist, 1)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.NotContains(t, client, "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq")
	assert.Contains(t, client, "select * from orders")
	assert.Contains(t, client, `fs.executeQuery({query: "pX6rtyrDUu2zGInsy0CSWuBNOko9LUWbMMfafE-d", text: "select * from orders where user_id IN (select id from users)", params: {}})`)
	assert.NotContains(t, client, `server(`)
	assert.NotContains(t, client, "ctx.executeQuery(;")

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq")
	assert.NotContains(t, server, "pX6rtyrDUu2zGInsy0CSWuBNOko9LUWbMMfafE-d")
	assert.Contains(t, server, `ctx.executeQuery({query: "mrC_twNRMS4kJCzTtyCUoyQhzK4H7T6tFAqEmdNq", text: "select id from users", params: {}})`)
	assert.Contains(t, server, `server(`)
}

func TestQueryOnGetClientDirect(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "import {getClient} from \"sqljoy\";\ngetClient().executeQuery(sql`select 1`);\n",
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select 1",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 2.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 2.0,
				"fileName": "app.js",
			},
		},
		"id": "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `getClient().executeQuery({query: "girgfUeDFYvBkSu2I-UQfMkALVGeEUOpwgDtbuGL", text: "select 1", params: {}})`)
}

//func TestInlineTernayExpression(t *testing.T) {
//	result := build(map[string]string{
//		"/app.js": "let query = sql`select * from users order by name ${window.sort ? sql`ASC` : sql`DESC`}`;\nfs.executeQuery(query);",
//	}, &api.FlowStateOptions{})
//
//	assert.Empty(t, result.Errors)
//	assert.Len(t, result.OutputFiles, 0)
//	assert.Len(t, result.Errors, 1)
//
//	client := string(getOutFile(&result, "client.bundle.js"))
//	assert.Contains(t, client, `sql.merge(query = {query: "bS2hbrOPG3ufGqac1Ir3KV43cjdgtaaFhGMMQ04I", text: "select * from users order by name $1", params: {}}, {query: "MjsIfgr8vMR3KZ-uu6SLDGKWJvrVuasRvNtHRcBk", text: "ASC", params: {}}, {query: "mE2k_mkwrNk4b252CyCYu2qUSfFfDyDVElTXquM_", text: "DESC", params: {}})`)
//}

//func TestInlineLogicalExpression(t *testing.T) {
//	result := build(map[string]string{
//		"/app.js": "let query = sql`select * from users order by name ${window.sort && sql.p`ASC` || sql.p`DESC`}`;\nfs.executeQuery(query);",
//	}, &api.FlowStateOptions{})
//
//	assert.Empty(t, result.Errors)
//	assert.Len(t, result.OutputFiles, 0)
//	assert.Len(t, result.Errors, 1)
//
//	client := string(getOutFile(&result, "client.bundle.js"))
//	assert.Contains(t, client, `sql.merge(query = {query: "bS2hbrOPG3ufGqac1Ir3KV43cjdgtaaFhGMMQ04I", text: "select * from users order by name $1", params: {}}, {query: "MjsIfgr8vMR3KZ-uu6SLDGKWJvrVuasRvNtHRcBk", text: "ASC", params: {}}, {query: "mE2k_mkwrNk4b252CyCYu2qUSfFfDyDVElTXquM_", text: "DESC", params: {}})`)
//}
