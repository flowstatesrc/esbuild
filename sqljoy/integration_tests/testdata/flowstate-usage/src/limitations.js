
import * as reexported from "./reexported";

((query) => {
	fs.executeQuery(query)
})(sql`-- we don't track queries through function calls, even if they could be known at compile-time`);

// Queries must be unconditional, even if the condition is known at compile-time.
// This could be resolved through a pre-pass step with another tool that removes the conditional before invoking flowstate.
let query;
if (process.env.PRODUCTION) {
	query = sql`-- prod query`
} else {
	query = sql`-- not-prod query`
}
fs.executeQuery(query);

function makeQuery() {
	return sql`-- knowable at compile time`;
}

// We don't evaluate functions, even if they could be evaluated at compile-time.
query = makeQuery();
fs.executeQuery(query);
