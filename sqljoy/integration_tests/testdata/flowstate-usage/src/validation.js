
import foo, {baz} from "./nested/other/validators";
import * as val from "./nested/other/validators";

export const arrow_validator = () => {};

export function func_validator() {

}

export function valid_but_unused() {}

function whatever() {
	fs.executeQuery(sql`select * from users`, {}, arrow_validator, func_validator);

	// Importing validators works as expected
	fs.executeQuery(sql`update users set name = ${fullName}`, {}, foo, val.bar, baz);

	// This validator will be ignored because it's been given as the params argument
	fs.executeQuery(sql`select * from users where 1=1`, valid_but_unused);
}
