import * as someModule from "./reexported.js"
import {foo as bar} from "./nested/other/functions.js"

export async function addMul(ctx, a, b, c) {
    return a*b + c;
}

const foo=7, arrowFun = async (ctx, x, y) => x - y, baz=12;

export let assignFunc = async function(ctx, m, n) {
  return m + n;
};

async function doStuff(a, b) {
    const point = await bar(fs.beginTx(), 12, 42);
    const result = await addMul(fs.beginTx(), a, point[0], point[1]);

    const result2 = await arrowFun(fs.beginTx(), 42, result);

    let promise = assignFunc(fs.beginTx(), 10, result2);

    const result3 = await someModule.someFunc(fs.beginTx());
    return result3;
}