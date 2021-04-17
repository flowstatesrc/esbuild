

const SORT_COLS = {
  "product_id": sql`product_id`,
  "order_id": sql`order_id`,
  "customer_id": sql`customer_id`,
  "total": sql`total`,
  "created_at": sql`created_at`
};

function getOrders(filterCol, value, sortCol, sortDir) {
    let filter;
    if (filterCol == "product_id") {
        filter = sql`o.product_id = ${value}`;
    } else if (filterCol == "order_id") {
        filter = sql`o.order_id = ${value}`;
    } else if (filterCol == "customer_id") {
        filter = sql`o.customer_id = ${value}`;
    } else {
        filter = sql`o.shipped_at = %{lateBound}`;
    }

    const orderByDir = (sortDir == null || sortDir < 0) ? sql`DESC` : sql`ASC`;
    const orderBy = SORT_COLS[sortCol];
    const orderClause = sql`ORDER BY o.${orderBy} ${orderByDir}`;

    const query = sql`SELECT * FROM orders AS o WHERE ${filter} ${orderClause}`;

    const lateBound = new Date();
    return fs.executeQuery(query, {lateBound});
}

/* Compiles to:

const SORT_COLS = {
  "product_id": {
    "query": "e2abff11934925135b6db4153b319d964c5cd6186fcc0e2833ac328a0cc0231e",
    "text": "product_id",
  },
  "order_id": {
    "query": "a71c534c8a3729b56f9e3380c732574a627c8138cd90756518f5e2efad057d43,
    "text": "order_id",
  },
  "customer_id": {
    "query": "66b71d7e1e98f250d6da3990bb34d51591010c27c932852c049cbf9310f8eca0",
    "text": "customer_id",
  },
  "total": {
    "query": "5058b1fe8bf9fffc57d94148a7ec55119c5cd9b21aa267cb13518bec0244241b",
    "text": "customer_id",
  },
  "created_at": {
    "query": "aba421c2c6fbf061c529a366c9538660f51e58656b94edba5cac3ee2aabaf38d",
    "text": "customer_id",
  },
};

function getOrders(filterCol, value, sortCol, sortDir) {
    let filter;
    if (filterCol == "product_id") {
        filter = {
          "query": "ef24d0d3c6239d824dc0f010f629f413849c468ef8f8a5d94ee83a31cda4f3d4",
          "text": "o.product_id = $0",
          "params": {
            "value": value,
          }
        };
    } else if (filterCol == "order_id") {
        filter = {
          "query": "68e48e79cf9ff3a30dae22113a7772d2eb0242967037318bc39e52b6a6b0ee6c",
          "text": "o.order_id = $0",
          "params": {
            "value": value,
          }
        };
    } else if (filterCol == "customer_id") {
      filter = {
        "query": "d3379c32a90fdf9382166f8f48034c459a8cc433730bc9476d39d9082c94583b",
        "text": "o.customer_id = $0",
        "params": {
          "value": value,
        }
      };
    } else {
      filter = {
        "query": "497e194b8a7d72f8cbab8264bbbdff00c52070005e0a948317b9edd1738a4332",
        "text": "o.shipped_at = $0",
        "params": {
          "lateBound": undefined,
        },
      };
    }

    const orderByDir = (sortDir == null || sortDir < 0) ? {
      "query": "3d568164de25ae2ab8f1ef2e55e7a925bf0a49af1fd85817c5113fcea8705750",
      "text": "DESC",
    } : {
      "query": "3abdbd673cbafe6194b8805f49af63728adea4ad0bb1f57e32145da43ab4c031",
      "text": "ASC",
    };
    const orderBy = SORT_COLS[sortCol];
    const orderClause = __mergeQueries({
      "query": "2b7e75b49512953cd29f3939724d242e880b262d169d6014bd8c64eb48c5781b",
      "text": "ORDER BY o.%{0}, %{1}"
    }, orderBy, orderByDir);

    const query = __mergeQueries({
      "query": "dfd38c3c4fc13c02025da5330886a37f29aedbec22bb9fcc45a2c0c73d89aac9",
      "text": "SELECT * FROM orders AS o WHERE %{0} %{1}"
    }, filter, orderClause);

    const lateBound = new Date();
    return fs.executeQuery(query, {lateBound});
}

*/