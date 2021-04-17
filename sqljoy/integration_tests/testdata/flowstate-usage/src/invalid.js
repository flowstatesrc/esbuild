
import {valid_but_unused} from "./validation";

executeQuery(sql`-- executeQuery calls not written as a method access are ignored completely`);

// We don't support commonjs imports, use ES6 imports
const nested = require("./nested/");
fs.executeQuery(nested.ONE_LEVEL_DEEP);

// We ignore validators only referenced by invalid queries
fs.executeQuery(invalid_query, {}, valid_but_unused);

function not_exported() {

}

function foo(){
	function not_exportable() {}

	fs.executeQuery(sql`select * from users where foo = ${bar}`, {}, not_exportable);

	fs.executeQuery(sql`select * from users where bar = ${foo}`, {}, not_exported);
}
