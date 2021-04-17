package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/evanw/esbuild/pkg/api"
)

func TestNoEvalFunction(t *testing.T) {
	result := build(map[string]string{
		"/app.js": `

		function makeQuery() {
			return sql` + "`-- knowable at compile time`" + `;
		}

		window.foo = function(){
			const query = makeQuery();
			fs.executeQuery(query);
		}
		`,
	}, &api.FlowStateOptions{})

	assert.NotEmpty(t, result.Errors)
	assert.Len(t, result.OutputFiles, 0)
	assert.Len(t, result.Errors, 1)

	assert.Equal(t, "could not identify query for first argument to executeQuery", result.Errors[0].Text)
}

func TestPassThroughFunctionArgs(t *testing.T) {
	result := build(map[string]string{
		"/app.js": `

		(function(query){
			return fs.executeQuery(query);
		})(` + "`select 1`);",
	}, &api.FlowStateOptions{})

	assert.NotEmpty(t, result.Errors)
	assert.Len(t, result.OutputFiles, 0)
	assert.Len(t, result.Errors, 1)

	assert.Equal(t, "could not identify query for first argument to executeQuery", result.Errors[0].Text)
}