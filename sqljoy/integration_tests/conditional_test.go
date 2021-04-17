package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/evanw/esbuild/pkg/api"
)

const prog = `const SORT_COLS = {
  "product_id": sql.p` + "`product_id`" + `,
  "order_id": sql.p` + "`order_id`" + `,
  "customer_id": sql.p` + "`customer_id`" + `,
  "total": sql.p` + "`total`" + `,
  "created_at": sql.p` + "`created_at`" + `
};

function getOrders(filterCol, value, sortCol, sortDir) {
    let filter;
    if (filterCol == "product_id") {
        filter = sql.p` + "`o.product_id = ${value}`" + `;
    } else if (filterCol == "order_id") {
        filter = sql.p` + "`o.order_id = ${value}`" + `;
    } else if (filterCol == "customer_id") {
        filter = sql.p` + "`o.customer_id = ${value}`" + `;
    } else {
        filter = sql.p` + "`o.shipped_at = %{lateBound}`" + `;
    }

    const orderByDir = (sortDir == null || sortDir < 0) ? sql.p` + "`DESC` : sql.p`ASC`" + `;
    const orderBy = SORT_COLS[sortCol];
    const orderClause = sql.p` + "`ORDER BY o.${orderBy} ${orderByDir}`" + `;

    const query = sql` + "`SELECT * FROM orders AS o WHERE ${filter} ${orderClause}`" + `;

    const lateBound = new Date();
    return fs.executeQuery(query, {lateBound});
}

getOrders(window.filterCol, window.value, window.sortCol, window.sortDir);
`

func TestConditionalFragments(t *testing.T) {
	result := build(map[string]string{
		"/app.js": prog,
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "SELECT * FROM orders AS o WHERE ${fragment1} ORDER BY o.${fragment2} ${fragment3}",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 25.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 28.0,
				"fileName": "app.js",
			},
		},
		"id": "7CsLDp-W4FRhLjxyFRuJYQ-0sqt1RvnyFvHc9MMG",
		"fragments": []interface{}{
			[]interface{}{
				map[string]interface{}{
					"id":       "-0tcmJGPljHBFkS6G1jCyU0J5_ozJxAUCLhX3MDS",
					"query":    "o.product_id = $1",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     12.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id":       "IFA8xH25nGEvbvo-y74kExORI9WfiWVT1LpPmz_w",
					"query":    "o.order_id = $1",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     14.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id":       "2Oxe6Mct_J91HL4F3hgya17FI9iO_F4pJ4GTPJYb",
					"query":    "o.customer_id = $1",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     16.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id":       "IW9pTVDZwdhBGSzYH_70oCO0AWw2TsksiGVS18CZ",
					"query":    "o.shipped_at = $1",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     18.0,
						"fileName": "app.js",
					},
				},
			},
			[]interface{}{
				map[string]interface{}{
					"id":       "w62tT4GzRhVydnLmwscqIFmd1PgNfuKTnbDuPbDr",
					"query":    "product_id",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     2.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id":       "yhOmssllGzhB_J_-Jaek_M6jCiLK8eiBKhBjG5WF",
					"query":    "order_id",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     3.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id":       "HjjWfb6PR9JE4Lqf8nCzTmi4fFu16shs62RH9rSg",
					"query":    "customer_id",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     4.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id":       "ESOYctF4cprMnlrq9_wAGg_9h4ySIzC-Q3Ermzvg",
					"query":    "total",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     5.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id":       "6Hip2aFyGRkHcxVQTAGgiPI2D8uywInKZfqsBfoE",
					"query":    "created_at",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     6.0,
						"fileName": "app.js",
					},
				},
			},
			[]interface{}{
				map[string]interface{}{
					"id": "mE2k_mkwrNk4b252CyCYu2qUSfFfDyDVElTXquM_",
					"query":    "DESC",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     21.0,
						"fileName": "app.js",
					},
				},
				map[string]interface{}{
					"id": "MjsIfgr8vMR3KZ-uu6SLDGKWJvrVuasRvNtHRcBk",
					"query":    "ASC",
					"isPublic": true,
					"clientReferences": 1.0,
					"definedAt": map[string]interface{}{
						"line":     21.0,
						"fileName": "app.js",
					},
				},
			},
		},
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	code := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, code, `product_id: {query: "w62tT4GzRhVydnLmwscqIFmd1PgNfuKTnbDuPbDr", text: "product_id", params: {}},`)
	assert.Contains(t, code, `order_id: {query: "yhOmssllGzhB_J_-Jaek_M6jCiLK8eiBKhBjG5WF", text: "order_id", params: {}},`)
	assert.Contains(t, code, `customer_id: {query: "HjjWfb6PR9JE4Lqf8nCzTmi4fFu16shs62RH9rSg", text: "customer_id", params: {}},`)
	assert.Contains(t, code, `total: {query: "ESOYctF4cprMnlrq9_wAGg_9h4ySIzC-Q3Ermzvg", text: "total", params: {}},`)
	assert.Contains(t, code, `created_at: {query: "6Hip2aFyGRkHcxVQTAGgiPI2D8uywInKZfqsBfoE", text: "created_at", params: {}}`)

	assert.Contains(t, code, `filter = {query: "-0tcmJGPljHBFkS6G1jCyU0J5_ozJxAUCLhX3MDS", text: "o.product_id = $1", params: {$1: value}};`)
	assert.Contains(t, code, `filter = {query: "IFA8xH25nGEvbvo-y74kExORI9WfiWVT1LpPmz_w", text: "o.order_id = $1", params: {$1: value}};`)
	assert.Contains(t, code, `filter = {query: "2Oxe6Mct_J91HL4F3hgya17FI9iO_F4pJ4GTPJYb", text: "o.customer_id = $1", params: {$1: value}};`)
	assert.Contains(t, code, `filter = {query: "IW9pTVDZwdhBGSzYH_70oCO0AWw2TsksiGVS18CZ", text: "o.shipped_at = $1", params: {lateBound: "__PARAM_"}};`)

	assert.Contains(t, code, `const orderByDir = sortDir == null || sortDir < 0 ? {query: "mE2k_mkwrNk4b252CyCYu2qUSfFfDyDVElTXquM_", text: "DESC", params: {}} : {query: "MjsIfgr8vMR3KZ-uu6SLDGKWJvrVuasRvNtHRcBk", text: "ASC", params: {}};`)
	assert.Contains(t, code, `sql.merge({query: "7CsLDp-W4FRhLjxyFRuJYQ-0sqt1RvnyFvHc9MMG", text: "SELECT * FROM orders AS o WHERE ${fragment1} ORDER BY o.${fragment2} ${fragment3}", params: {}}, filter, orderBy, orderByDir);`)
	assert.Contains(t, code, `return fs.executeQuery(query, {lateBound});`)
}
