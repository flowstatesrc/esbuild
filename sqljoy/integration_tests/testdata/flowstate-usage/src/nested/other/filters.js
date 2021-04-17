

export const where = sql`WHERE %{SESSION.role} = 'admin'`;

export const NESTED2 = sql`-- Comments are allowed
	UPDATE nested SET foo = %{bar}`;

export const withComment = sql`/* C-style comments are also supported 
   and can span multiple lines */ INSERT INTO nested (name) VALUES (%{name})`;