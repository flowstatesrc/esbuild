package integration_tests

import (
"testing"

"github.com/stretchr/testify/assert"
)

const nested = `
export function nested(tx, c1, c2, c3, c4, both, sortDir) {
		cond1 =  sql.p` + "`${sql.p(c1.name)} ${sql.p(c1.eq ? '=' : '!=')} ${c1.value}`" + `;
		cond2 =  sql.p` + "`${sql.p(c2.name)} ${sql.p(c2.eq ? '=' : '!=')} ${c2.value}`" + `;
		cond3 =  sql.p` + "`${sql.p(c3.name)} ${sql.p(c3.eq ? '=' : '!=')} ${c3.value}`" + `;
		cond4 =  sql.p` + "`${sql.p(c4.name)} ${sql.p(c4.eq ? '=' : '!=')} ${c4.value}`" + `;

		and = sql.p` + "`${c1.enabled ? cond1 : cond2} AND ${c3.enabled ? cond3 : cond3}`" + `;
		or = sql.p` + "`${c2.enabled ? cond2 : cond3} OR ${c4.enabled ? cond4 : cond1}`" + `;

		let query = sql` + "`SELECT * FROM orders WHERE ${both ? or : and} ORDER BY created_at ${sql.p(sortDir)}`" + `;
    return tx.executeQuery(query);
}

nested(fs.beginTx(), ...window.args);
`

func TestNestedFragments(t *testing.T) {
	result := build(map[string]string{
		"/app.js": nested,
	}, nil)

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.Empty(t, whitelist)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.NotContains(t, client, "nested")
	assert.NotContains(t, client, "cond1")
	assert.NotContains(t, client, "c1")
	assert.Contains(t, client, `fs.serverCall("EALxv89d3vuX00lFTAuaCUy4o-1D9Bb68NA7Dhm7", ...window.args)`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `cond1 = {query: "bBefIeb2K2KQVdirQPRU7QLki2hWORNHO4V9Njji", text: "$1 $2 $3", params: {$1: sql.p(c1.name), $2: sql.p(c1.eq ? "=" : "!="), value: c1.value}};`)
	assert.Contains(t, server, `cond2 = {query: "bBefIeb2K2KQVdirQPRU7QLki2hWORNHO4V9Njji", text: "$1 $2 $3", params: {$1: sql.p(c2.name), $2: sql.p(c2.eq ? "=" : "!="), value: c2.value}};`)
	assert.Contains(t, server, `cond3 = {query: "bBefIeb2K2KQVdirQPRU7QLki2hWORNHO4V9Njji", text: "$1 $2 $3", params: {$1: sql.p(c3.name), $2: sql.p(c3.eq ? "=" : "!="), value: c3.value}};`)
	assert.Contains(t, server, `cond4 = {query: "bBefIeb2K2KQVdirQPRU7QLki2hWORNHO4V9Njji", text: "$1 $2 $3", params: {$1: sql.p(c4.name), $2: sql.p(c4.eq ? "=" : "!="), value: c4.value}};`)
	assert.Contains(t, server, `and = sql.merge({query: "B4LMwzykPazjHC-a7xqjp-OPJduYtwsODLeCc2a9", text: "%{} AND %{}", params: {}}, c1.enabled ? cond1 : cond2, c3.enabled ? cond3 : cond3);`)
	assert.Contains(t, server, `or = sql.merge({query: "1tXJ_klVv7PlxVxh2Lf2AoIKIU8VO6sd-9foxkLt", text: "%{} OR %{}", params: {}}, c2.enabled ? cond2 : cond3, c4.enabled ? cond4 : cond1);`)
	assert.Contains(t, server, `let query = sql.merge({query: "bsxJbJaYaHxS4NNz-F7qWslcfQnMSrT0j_-u5UAv", text: "SELECT * FROM orders WHERE %{} ORDER BY created_at $1", params: {$1: sql.p(sortDir)}}, both ? or : and);`)
	assert.Contains(t, server, `return tx.executeQuery(query);`)
}

