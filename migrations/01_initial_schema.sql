DROP TYPE IF EXISTS transaction_status CASCADE;
CREATE TYPE transaction_status AS ENUM('new', 'batched', 'success', 'error');

DROP TYPE IF EXISTS batch_status CASCADE;
CREATE TYPE batch_status AS ENUM('new', 'pending', 'success', 'error');

DROP TABLE IF EXISTS transaction;
CREATE TABLE transaction (
	id BIGSERIAL PRIMARY KEY,
	order_id TEXT NOT NULL,
	wallet TEXT NOT NULL,
	amount REAL NOT NULL,
	"comment" TEXT,
	status transaction_status NOT NULL DEFAULT 'new',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	UNIQUE (order_id, wallet, amount, "comment")
);

DROP TABLE IF EXISTS batch;
CREATE TABLE batch (
	uuid UUID NOT NULL PRIMARY KEY,
	transaction_ids BIGINT[] NOT NULL DEFAULT '{}',
	status batch_status NOT NULL DEFAULT 'new',

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
