package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/evanw/esbuild/pkg/api"
)

func TestBasicServerFunc(t *testing.T) {
	const prog = `
	export async function addMul(ctx, a, b, c) {
		return a*b + c;
	}

	const point = [23.45,-74.56];
	window.doStuff = async function(a, b) {
		const result = await addMul(window.fs.beginTx(), a, point[0], point[1]);
	};
	`

	result := build(map[string]string{
		"/app.js": prog,
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, client, `await window.fs.serverCall("GUD_VBwlbW3JCeGXPfQWLkTtPJaAEes7TfM3_FDB", a, point[0], point[1]);`)
	assert.NotContains(t, client, `function addMul(ctx, a, b, c)`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `async function addMul(ctx, a, b, c)`)
	assert.Contains(t, server, `var functions = {
    GUD_VBwlbW3JCeGXPfQWLkTtPJaAEes7TfM3_FDB: addMul
  };`)
}

func TestArrowServerFunc(t *testing.T) {
	const prog = `
	export const foo=7, arrowFun = async (ctx, x, y) => x - y, baz=12;

	window.doStuff = async function(a, b) {
		const result = await arrowFun(window.fs.beginTx(), a, foo+42);
	};
	`

	result := build(map[string]string{
		"/app.js": prog,
	}, &api.FlowStateOptions{})

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, client, "foo = 7")
	assert.Contains(t, client, `await window.fs.serverCall("R0RKu9Nkzeli4YI4SQYYnEus7uJ9tEu1vZeF3f50", a, foo + 42);`)
	assert.NotContains(t, client, `async (ctx, x, y) => x - y`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, "foo = 7")
	assert.Contains(t, server, `arrowFun = async (ctx, x, y) => x - y`)
	assert.Contains(t, server, `var functions = {
    R0RKu9Nkzeli4YI4SQYYnEus7uJ9tEu1vZeF3f50: arrowFun
  };`)
}

func TestServerFunctionExpr(t *testing.T) {
	const prog = `
	import {removeMeRecursive} from "./foo"
	let removeMe = 42;

	const removeFunc = (n) => removeMeRecursive*n;

	export let assignFunc = async function(ctx, m, n) {
		return m + removeFunc(n) - removeMe;
	};

	window.doStuff = async function(a, b) {
		let promise = assignFunc(window.fs.beginTx(), a);
		return promise;
	};
	`

	result := build(map[string]string{
		"/foo.js": "export const removeMeRecursive = 2; export function notUsed() {}",
		"/app.js": prog,
	}, &api.FlowStateOptions{}, "/app.js")

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.OutputFiles)

	client := string(getOutFile(&result, "client.bundle.js"))
	assert.Contains(t, client, `promise = window.fs.serverCall("yWBgZK8_NCnEk1wxyV500RDaGDZhquVUWYLaBn0e", a);`)
	assert.NotContains(t, client, `assignFunc = async function(ctx, m, n)`)
	assert.NotContains(t, client, `42`)
	assert.NotContains(t, client, `removeMe`)
	assert.NotContains(t, client, `removeFunc`)
	assert.NotContains(t, client, `removeMeRecursive`)
	assert.NotContains(t, client, `notUsed`)

	server := string(getOutFile(&result, "server.bundle.js"))
	assert.Contains(t, server, `removeMe`)
	assert.Contains(t, server, `removeFunc`)
	assert.Contains(t, server, `removeMeRecursive`)
	assert.NotContains(t, server, `notUsed`)
	assert.Contains(t, server, `assignFunc = async function(ctx, m, n)`)
	assert.Contains(t, server, `var functions = {
    yWBgZK8_NCnEk1wxyV500RDaGDZhquVUWYLaBn0e: assignFunc
  };`)
}