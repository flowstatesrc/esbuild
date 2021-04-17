package integration_tests

// Tests for unsupported syntax of the "wontfix" variety. These will likely never be valid.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/evanw/esbuild/pkg/api"
)

func TestBareExecuteQuery(t *testing.T) {
	result := build(map[string]string{
		"/app.js": "executeQuery(sql`executeQuery calls not written as a method access are ignored completely`);\n",
	}, &api.FlowStateOptions{})

	assert.NotEmpty(t, result.Errors)
	assert.Len(t, result.OutputFiles, 0)
	assert.Equal(t, "executeQuery must be invoked as a method", result.Errors[0].Text)
}

func TestBareBeginTx(t *testing.T) {
	result := build(map[string]string{
		"/app.js": `

		export const foo = async (ctx) => {
			return 42;
		};
		window.promise = foo(beginTx());
		`,
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.Len(t, result.OutputFiles, 2)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.NotContains(t, server, "return 42;")
	assert.NotContains(t, server, "foo")
}

func TestNoDynamicImports(t *testing.T) {
	result := build(map[string]string{
		"/app.js": `
		const other = require("./other");
		fs.executeQuery(other.query);

		const other2 = import("./other");
		fs.executeQuery(other2.query);
		`,
		"/other.js": `

		export const query = sql` + "`select 1`;",
	}, &api.FlowStateOptions{}, "/app.js")

	assert.NotEmpty(t, result.Errors)
	assert.Len(t, result.Errors, 2)
	assert.Len(t, result.OutputFiles, 0)

	for i := range result.Errors {
		assert.Equal(t, "could not identify query for first argument to executeQuery", result.Errors[i].Text)
	}
}

func TestUnexportedValidators(t *testing.T) {
	result := build(map[string]string{
		"/app.js": `

		function not_exported(data, errors) {}

		window.foo = function(){
			function not_exportable(data, errors) {}

			fs.executeQuery(sql` + "`select * from users where foo = ${bar}`" + `, {}, not_exportable);
			
			fs.executeQuery(sql` + "`select * from users where bar = ${foo}`" + `, {}, not_exported);
		}
		`,
	}, &api.FlowStateOptions{})

	assert.NotEmpty(t, result.Errors)
	assert.Len(t, result.OutputFiles, 0)
	assert.Len(t, result.Errors, 2)

	assert.Equal(t, "server call not_exportable must refer to a top level exportable function", result.Errors[0].Text)
	assert.Equal(t, "function not_exported must be exported", result.Errors[1].Text)
}
