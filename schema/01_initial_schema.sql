CREATE SCHEMA IF NOT EXISTS sender;

DROP TYPE IF EXISTS sender.transfer_status CASCADE;
CREATE TYPE sender.transfer_status AS ENUM('new', 'batched', 'success', 'error');

DROP TYPE IF EXISTS sender.batch_status CASCADE;
CREATE TYPE sender.batch_status AS ENUM('new', 'processing', 'success', 'error');

DROP TABLE IF EXISTS sender.transfer CASCADE;
CREATE TABLE sender.transfer (
	id BIGSERIAL PRIMARY KEY,
	order_id TEXT NOT NULL,
	wallet TEXT NOT NULL,
	amount REAL NOT NULL,
	"comment" TEXT,
	status sender.transfer_status NOT NULL DEFAULT 'new',

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	UNIQUE (order_id, wallet, amount, "comment")
);

DROP TABLE IF EXISTS sender.batch CASCADE;
CREATE TABLE sender.batch (
	uuid UUID NOT NULL PRIMARY KEY,
	transfer_ids BIGINT[] NOT NULL DEFAULT '{}',
	status sender.batch_status NOT NULL DEFAULT 'new',

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DROP TABLE IF EXISTS sender.external_message CASCADE;
CREATE TABLE sender.external_message (
	uuid UUID NOT NULL PRIMARY KEY,
	wallet_hash TEXT NOT NULL,
	batch_uuid UUID NOT NULL,
	query_id INT NOT NULL,
	expired_at TIMESTAMP NOT NULL,
	created_at TIMESTAMP NOT NULL,

	CONSTRAINT fk_transaction_batch
        FOREIGN KEY (batch_uuid)
        REFERENCES sender.batch(uuid)
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS ind_last_external_message
	ON sender.external_message(wallet_hash, created_at DESC);

DROP TABLE IF EXISTS sender.transaction CASCADE;
CREATE TABLE sender.transaction (
	hash TEXT NOT NULL PRIMARY KEY,
	message_uuid UUID NOT NULL,

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT fk_transaction_message
        FOREIGN KEY (message_uuid)
        REFERENCES sender.external_message(uuid)
        ON DELETE RESTRICT
);

DROP TABLE IF EXISTS sender.wallet CASCADE;
CREATE TABLE sender.wallet (
	hash TEXT NOT NULL PRIMARY KEY,
	mainnet_address TEXT NOT NULL,
	testnet_address TEXT NOT NULL,
	last_proccessed_lt BIGINT NOT NULL DEFAULT 0,

	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)
