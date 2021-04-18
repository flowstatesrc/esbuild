package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicValidation(t *testing.T) {
	const prog = `

	export const arrow_validator = (data, errors) => {};
	export function func_validator(data, errors) {}
	export function valid_but_unused(data, errors) {}

	(function() {
		fs.executeQuery(sql` + "`select * from users`" + `, {}, arrow_validator, func_validator);
	})();
	`

	result := build(map[string]string{
		"/app.js": prog,
	}, nil)

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)

	assert.Equal(t, map[string]interface{}{
		"query": "select * from users",
		"type": "select",
		"isPublic": true,
		"clientReferences": 1.0,
		"definedAt": map[string]interface{}{
			"line": 8.0,
			"fileName": "app.js",
		},
		"usages": []interface{}{
			map[string]interface{}{
				"line": 8.0,
				"fileName": "app.js",
			},
		},
		"id": "xrN_yMcRbkp8nkZxzb7PpS4hId7y7sYdjr7Q_sI3",
	}, whitelist[0])
	assert.Len(t, whitelist, 1)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, client, `fs.executeQuery({query: "xrN_yMcRbkp8nkZxzb7PpS4hId7y7sYdjr7Q_sI3", text: "select * from users", params: {}}, {}, arrow_validator, func_validator)`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `var arrow_validator = (data, errors) => {
  };`)
	assert.Contains(t, server, `function func_validator(data, errors) {
  }`)
	assert.Contains(t, server, `var validators = {
    xrN_yMcRbkp8nkZxzb7PpS4hId7y7sYdjr7Q_sI3: (e, s) => {
      arrow_validator(e, s);
      func_validator(e, s);
    }
  };`)
}

func TestImportedValidation(t *testing.T) {
	const prog = `
	import {arrow_validator, func_validator, valid_but_unused} from "./validation";

	(function() {
		// valid_but_unused will be ignored because it's been given as the params argument
		fs.executeQuery(sql` + "`select * from users`" + `, valid_but_unused, arrow_validator, func_validator);
	})();
	`

	result := build(map[string]string{
		"/app.js": prog,
		"/validation.js": `
			export const arrow_validator = (data, errors) => {};
			export function func_validator(data, errors) {}
			export function valid_but_unused(data, errors) {}
		`,
	}, nil, "app.js")

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)
	whitelist := getClientWhitelist(&result)
	assert.NotEmpty(t, whitelist)
	assert.Len(t, whitelist, 1)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, client, `fs.executeQuery({query: "xrN_yMcRbkp8nkZxzb7PpS4hId7y7sYdjr7Q_sI3", text: "select * from users", params: {}}, valid_but_unused, arrow_validator, func_validator)`)
	assert.Contains(t, client, `var arrow_validator = (data, errors) => {`)
	assert.Contains(t, client, `function func_validator(data, errors) {`)
	assert.Contains(t, client, `function valid_but_unused(data, errors) {`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `var arrow_validator = (data, errors) => {
  };`)
	assert.Contains(t, server, `function func_validator(data, errors) {
  }`)
	assert.Contains(t, server, `var validators = {
    xrN_yMcRbkp8nkZxzb7PpS4hId7y7sYdjr7Q_sI3: (e, s) => {
      arrow_validator(e, s);
      func_validator(e, s);
    }
  };`)
	assert.NotContains(t, server, "function valid_but_unused(data, errors)")
}
