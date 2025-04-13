package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	DuplicateKeyValue string = "23505"
)

var (
	ErrDuplicateKeyValue = errors.New("duplicate key value")
)

func (p *Postgres) PersistMessage(ctx context.Context, msg types.SendTONMessage) (
	int64, error) {

	fields := []string{"order_id", "wallet", "amount", "comment"}
	rows := make([][]any, len(msg.Data))

	for i, tr := range msg.Data {
		rows[i] = []any{msg.TransactionID, tr.Wallet, tr.Amount, tr.Comment}
	}

	p.log.Debug("COPY", "fields", fields, "rows", rows)
	inserted, err := p.pg.CopyFrom(ctx, pgx.Identifier{"transfer"}, fields,
		pgx.CopyFromRows(rows))
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			// if for some reason
			if pgErr.Code == DuplicateKeyValue {
				return 0, ErrDuplicateKeyValue
			}
		}
		return 0, fmt.Errorf("couldn't persist tx: %w", err)
	}

	if inserted == 0 {
		return 0, fmt.Errorf("persist msg: %v, no rows inserted", msg)
	}

	return inserted, nil
}

func (p *Postgres) NextBatch(ctx context.Context, maxItems int) (uuid.UUID, error) {

	var batchUUID string

	// atomically get up to maxItems new transactions, mark them as batched
	// and create a new row in the batch table with all the transaction ids that
	// were picked for this batch
	stmt := `
	WITH batched_transfers AS (
		SELECT
			id, order_id, wallet, amount, "comment"
		FROM
			transfer
		WHERE
			status = 'new'
		ORDER BY updated_at
		LIMIT @max_batch_size
	), updated AS (
		UPDATE
			transfer t
		SET
			status = 'batched', updated_at = NOW()
		FROM batched_transfers b
		WHERE
			t.order_id = b.order_id AND
			t.wallet = b.wallet AND
			t.amount = b.amount AND
			t.comment = b.comment
		RETURNING t.id
	), batch_tx_ids AS (
		SELECT @batch_uuid::uuid as uuid, ARRAY_AGG(id) as transfer_ids
		FROM updated
	), batch AS (
		INSERT INTO batch (uuid, transfer_ids)
		SELECT uuid, transfer_ids
		FROM batch_tx_ids
		WHERE EXISTS (SELECT 1 FROM updated)
		RETURNING uuid
	)
	SELECT * FROM batch
	`

	generatedUUID := uuid.New()

	err := p.pg.QueryRow(ctx, stmt, pgx.NamedArgs{
		"max_batch_size": maxItems,
		"batch_uuid":     generatedUUID.String(),
	}).Scan(&batchUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, nil
		} else {
			p.log.Error("batch create failed", "error", err)
			return uuid.UUID{}, fmt.Errorf("batch create error: %w", err)
		}
	}

	p.log.Debug("New batch UUID", "batch", generatedUUID)

	return generatedUUID, nil
}

func (p *Postgres) GetNewBatches(ctx context.Context) ([]uuid.UUID, error) {
	p.log.Debug("Fetching new batches")

	var newBatches []uuid.UUID

	stmt := `SELECT uuid FROM batch WHERE status = 'new' ORDER BY created_at`
	rows, err := p.pg.Query(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("couldn't get new batches: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		newBatches = append(newBatches, id)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return newBatches, nil
}

func (p *Postgres) GetTransfersBatch(ctx context.Context, batchUUID uuid.UUID) (
	*types.Batch, error) {
	p.log.Debug("Fetching batch transfers")

	stmt := `
	SELECT
		id, order_id, wallet, amount, comment
	FROM transfer
	WHERE id IN (
		SELECT UNNEST(transfer_ids)
		FROM batch WHERE uuid = @batch_uuid
	)
	ORDER BY id`

	rows, err := p.pg.Query(ctx, stmt, pgx.NamedArgs{
		"batch_uuid": batchUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get new batches: %w", err)
	}

	defer rows.Close()

	transfers, err := pgx.CollectRows(rows, pgx.RowToStructByName[types.Transfer])
	if err != nil {
		return nil, fmt.Errorf("transfers parsing error: %w", err)
	}

	return &types.Batch{
		UUID:      batchUUID,
		Transfers: transfers,
	}, nil
}

func (p *Postgres) UpdateBatchStatus(ctx context.Context, batchUUID uuid.UUID,
	status types.BatchStatus) error {
	p.log.Debug("Updating batch status", "uuid", batchUUID, "status", status)

	stmt := `
	UPDATE batch
	SET status = @status
	WHERE uuid = @batch_uuid
	`

	resp, err := p.pg.Exec(ctx, stmt, pgx.NamedArgs{
		"status":     status,
		"batch_uuid": batchUUID,
	})
	if err != nil {
		return fmt.Errorf(
			"updating batch %s status %s error: %w", batchUUID, status, err)
	}

	if resp.RowsAffected() == 0 {
		return fmt.Errorf(
			"updating batch %s status %s: no rows affected", batchUUID, status)
	}

	return nil
}

func (p *Postgres) GetLastQueryID(ctx context.Context, walletHash string) (
	uint64, error) {
	p.log.Debug("Fetching last query id")

	var queryID int64

	stmt := `
	SELECT query_id
	FROM transaction
	WHERE wallet_hash = @wallet_hash
	ORDER BY created_at DESC
	LIMIT 1
	`

	err := p.pg.QueryRow(ctx, stmt, pgx.NamedArgs{
		"wallet_hash": walletHash,
	}).Scan(&queryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		} else {
			p.log.Error(
				"fetching last query id failed",
				"wallet", walletHash,
				"error", err,
			)
			return 0, fmt.Errorf("GetLastID error: %w", err)
		}
	}

	return uint64(queryID), nil

}

func (p *Postgres) PersistWallet(ctx context.Context, hash, mainnetAddress,
	testnetAddress string) error {
	p.log.Debug("Persisting wallet", "hash", hash)

	stmt := `
	INSERT INTO wallet (hash, mainnet_address, testnet_address)
	VALUES (@hash, @mainnet_address, @testnet_address)
	ON CONFLICT (hash)
	DO NOTHING
	`

	_, err := p.pg.Exec(ctx, stmt, pgx.NamedArgs{
		"hash":            hash,
		"mainnet_address": mainnetAddress,
		"testnet_address": testnetAddress,
	})
	if err != nil {
		return fmt.Errorf(
			"inserting wallet %s error: %w", hash, err)
	}

	return nil
}
