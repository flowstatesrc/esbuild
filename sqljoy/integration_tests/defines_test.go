package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientProcessEnvAdded(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "let query = sql`select * from users where id = ${ENV_ACCOUNT_ID}`;\nif (!ENV_SERVER) { fs.executeQuery(query); }",
	}, nil)

	assert.Empty(t, result.Errors)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)
	assert.Len(t, result.OutputFiles, 3)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, client, `{query: "HbSIDVYUZouRpNqTjEqMHpRhuaDCgzscWVYbO5Nd", text: "select * from users where id = $1", params: {$1: "account-id"}}`)

	server := string(getOutFile(&result, "server.bundle.js"))
	_ = server // TODO
}

func TestServerProcessEnvAdded(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "let query = sql`select * from users where id = ${ENV_ACCOUNT_ID}`;" + `
			export const arrowFun = async (ctx, x, y) => ENV_SERVER ? ctx.executeQuery(query) : null;
			arrowFun(fs.beginTx(), a, foo+42);`,
	}, nil)

	assert.Empty(t, result.Errors)
	whitelist := getClientWhitelist(&result)
	assert.Empty(t, whitelist)
	serverWhitelist := getServerWhitelist(&result)
	assert.NotEmpty(t, serverWhitelist)

	assert.Len(t, result.OutputFiles, 3)


	client := string(getOutFile(&result, "client.bundle.js"))
	_ = client // TODO

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `{query: "HbSIDVYUZouRpNqTjEqMHpRhuaDCgzscWVYbO5Nd", text: "select * from users where id = $1", params: {$1: "account-id"}}`)
}