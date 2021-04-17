
import {where} from "./filters";
export * as filters from "./filters";

const table = sql`nested`;

// One can build queries from fragments defined or imported from elsewhere,
// provided that they can be statically and unconditionally composed.
export const NESTED = sql`SELECT nested1 FROM ${table} ${where}`;

// Re-exports are fine
export {NESTED2} from "./filters";

export const NESTED3 = sql`SELECT 3 FROM ${table}`;

export const NESTED4 = sql`SELECT 4 FROM ${table}`;

export async function someFunc(ctx) {
  return ctx.foo();
}