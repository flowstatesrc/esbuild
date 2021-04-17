import {sql} from "flowstate"

const ONE_LEVEL_DEEP = sql`SELECT * FROM whatever -- multiline queries are fine
	WHERE id = %{id}`;

export default ONE_LEVEL_DEEP;
