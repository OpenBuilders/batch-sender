DROP TYPE IF EXISTS transfer_status CASCADE;
CREATE TYPE transfer_status AS ENUM('new', 'batched', 'success', 'error');

DROP TYPE IF EXISTS batch_status CASCADE;
CREATE TYPE batch_status AS ENUM('new', 'pending', 'success', 'error');

DROP TABLE IF EXISTS transfer CASCADE;
CREATE TABLE transfer (
	id BIGSERIAL PRIMARY KEY,
	order_id TEXT NOT NULL,
	wallet TEXT NOT NULL,
	amount REAL NOT NULL,
	"comment" TEXT,
	status transfer_status NOT NULL DEFAULT 'new',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	UNIQUE (order_id, wallet, amount, "comment")
);

DROP TABLE IF EXISTS batch CASCADE;
CREATE TABLE batch (
	uuid UUID NOT NULL PRIMARY KEY,
	transfer_ids BIGINT[] NOT NULL DEFAULT '{}',
	status batch_status NOT NULL DEFAULT 'new',

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DROP TABLE IF EXISTS transaction CASCADE;
CREATE TABLE transaction (
	hash TEXT NOT NULL PRIMARY KEY,
	batch_uuid UUID NOT NULL,
	query_id INT NOT NULL,
	wallet_hash TEXT NOT NULL,
	is_testnet BOOL NOT NULL DEFAULT true,
	success BOOL NOT NULL,
	error TEXT NOT NULL DEFAULT '',

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT fk_transaction_batch
        FOREIGN KEY (batch_uuid)
        REFERENCES batch(uuid)
        ON DELETE RESTRICT,

	CONSTRAINT fk_transaction_wallet
        FOREIGN KEY (wallet_hash)
        REFERENCES wallet(hash)
        ON DELETE RESTRICT
);

DROP TABLE IF EXISTS wallet CASCADE;
CREATE TABLE wallet (
	hash TEXT NOT NULL PRIMARY KEY,
	mainnet_address TEXT NOT NULL,
	testnet_address TEXT NOT NULL,

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)
