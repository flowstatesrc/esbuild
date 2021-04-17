
import {NESTED as NESTED_ALIAS} from "./nested/other";
import ONE_LEVEL_DEEP from "./nested";
import * as reexported from "./reexported";


function foo(fs, bar) {
	const query = sql`SELECT * FROM local_var WHERE bar = ${bar}`;

	const closure = (baz) => {
		// Queries can be specified inline
		fs.executeQuery(sql`SELECT * FROM foo WHERE baz = ${baz} AND bar = ${bar} ORDER BY bar`);
		// Or declared in any parent scope
		fs.executeQuery(query);
	};

	// Or from an import
	fs.executeQuery(NESTED_ALIAS);
	// Or from a namespaced import
	fs.executeQuery(reexported.ALIASED3);

	return closure;
}

// We don't actually check that the object executeQuery is called on is valid
doesNotExist.executeQuery(reexported.RENAMED);

// Default imports work as you'd expect
doesNotExist.executeQuery(ONE_LEVEL_DEEP);

// We can track a query or namespace across simple, unconditional variable assignment
const alias = reexported.nestedAlias;
const alias2 = alias;
fs.executeQuery(alias2.NESTED4);

// We can detect a query defined in an object literal
const queries = {
  'query': sql`SELECT * FROM object_literal WHERE ${query}`,
  'prop': sql`SELECT * FROM object_property`
};
function dynamic(key) {
  return fs.executeQuery(queries[key]);
}
fs.executeQuery(queries.prop);

// Multiple queries in one statement are permitted, but harder to audit. Prefer a CTE or calling executeQuery twice.
// Only the result of the last query will be returned.
fs.executeQuery(sql`select 1; select 2`);
