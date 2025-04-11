CREATE TYPE status AS ENUM('new', 'pending', 'success', 'error');

DROP TABLE IF EXISTS transaction;
CREATE TABLE transaction (
	id TEXT NOT NULL PRIMARY KEY,
	processed BOOL NOT NULL DEFAULT FALSE,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DROP TABLE IF EXISTS transfer;
CREATE TABLE transfer (
	transaction_id TEXT NOT NULL,
	wallet TEXT NOT NULL,
	amount REAL NOT NULL,
	"comment" TEXT,
	status status NOT NULL DEFAULT 'new',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	UNIQUE (transaction_id, wallet, amount, "comment")
);
