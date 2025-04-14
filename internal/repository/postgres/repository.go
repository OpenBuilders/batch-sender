package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

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
	inserted, err := p.pg.CopyFrom(ctx, pgx.Identifier{"sender", "transfer"}, fields,
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
	p.log.Debug("getting next batch", "size", maxItems)
	var batchUUID string

	// atomically get up to maxItems new transactions, mark them as batched
	// and create a new row in the batch table with all the transaction ids that
	// were picked for this batch
	stmt := `
	WITH batched_transfers AS (
		SELECT
			id, order_id, wallet, amount, "comment"
		FROM
			sender.transfer
		WHERE
			status = 'new'
		ORDER BY updated_at
		LIMIT @max_batch_size
	), updated AS (
		UPDATE
			sender.transfer t
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
	), new_batch AS (
		INSERT INTO sender.batch (uuid, transfer_ids)
		SELECT uuid, transfer_ids
		FROM batch_tx_ids
		WHERE EXISTS (SELECT 1 FROM updated)
		RETURNING uuid
	)
	SELECT * FROM new_batch
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

	stmt := `SELECT uuid FROM sender.batch WHERE status = 'new' ORDER BY created_at`
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
	FROM sender.transfer
	WHERE id IN (
		SELECT UNNEST(transfer_ids)
		FROM sender.batch WHERE uuid = @batch_uuid
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
	UPDATE sender.batch
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

func (p *Postgres) UpdateLastWalletLt(ctx context.Context, walletHash string,
	lt uint64) error {
	p.log.Debug("Updating last wallet lt", "wallet", walletHash, "lt", lt)

	stmt := `UPDATE sender.wallet SET last_processed_lt = @lt WHERE hash = @wallet_hash`

	resp, err := p.pg.Exec(ctx, stmt, pgx.NamedArgs{
		"wallet_hash": walletHash,
		"lt":          lt,
	})
	if err != nil {
		return fmt.Errorf(
			"updating wallet lt %s %d error: %w", walletHash, lt, err)
	}

	if resp.RowsAffected() == 0 {
		return fmt.Errorf(
			"updating wallet lt %s %d error: no rows affected", walletHash, lt)
	}

	return nil
}

func (p *Postgres) GetLastWalletLt(ctx context.Context, walletHash string) (
	uint64, error) {
	p.log.Debug("Fetching last wallet lt")

	var walletLt int64

	stmt := `SELECT last_processed_lt FROM sender.wallet WHERE hash = @wallet_hash`

	err := p.pg.QueryRow(ctx, stmt, pgx.NamedArgs{
		"wallet_hash": walletHash,
	}).Scan(&walletLt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		} else {
			p.log.Error(
				"fetching last wallet lt failed",
				"wallet", walletHash,
				"error", err,
			)
			return 0, fmt.Errorf("GetLastWalletLt error: %w", err)
		}
	}

	return uint64(walletLt), nil
}

func (p *Postgres) GetLastQueryID(ctx context.Context, walletHash string) (
	uint64, error) {
	p.log.Debug("Fetching last query id")

	var queryID int64

	stmt := `
	SELECT query_id
	FROM sender.external_message
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
	INSERT INTO sender.wallet (hash, mainnet_address, testnet_address)
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

func (p *Postgres) PersistTransaction(ctx context.Context, hash []byte,
	info *types.ExtMsgInfo) error {
	stmt := `
	INSERT INTO sender.transaction (hash, message_uuid)
	VALUES (@tx_hash, @msg_uuid)
	`

	tx_hash := base64.URLEncoding.EncodeToString(hash)

	_, err := p.pg.Exec(ctx, stmt, pgx.NamedArgs{
		"tx_hash":  tx_hash,
		"msg_uuid": info.UUID,
	})
	if err != nil {
		return fmt.Errorf("inserting tx %s %s error: %w", tx_hash, info.UUID, err)
	}

	return p.updateBatch(ctx, info.UUID)
}

func (p *Postgres) updateBatch(ctx context.Context, msgUUID uuid.UUID) error {
	p.log.Debug("Updating batch by msg uuid", "uuid", msgUUID)

	stmt := `
	WITH updated_batch AS (
		UPDATE sender.batch b
		SET status = 'success'
		FROM sender.external_message m
		WHERE
			b.uuid = m.batch_uuid AND
			m.uuid = @msg_uuid
		RETURNING b.transfer_ids
	)
	UPDATE sender.transfer
	SET status = 'success'
	WHERE id IN (
		SELECT UNNEST(transfer_ids)
		FROM updated_batch
	);
	`

	resp, err := p.pg.Exec(ctx, stmt, pgx.NamedArgs{
		"msg_uuid": msgUUID,
	})
	if err != nil {
		return fmt.Errorf(
			"updating batch by msg uuid %s %s error: %w", msgUUID, err)
	}

	if resp.RowsAffected() == 0 {
		return fmt.Errorf(
			"updating batch by msg uuid %s: no rows affected", msgUUID)
	}

	return nil
}

func (p *Postgres) MarkBatchAsProcessing(ctx context.Context, walletHash string,
	batchUUID uuid.UUID, info *types.ExtMsgInfo) error {

	stmt := `
	UPDATE sender.batch
	SET status = 'processing'
	WHERE uuid = @batch_uuid
	`

	_, err := p.pg.Exec(ctx, stmt, pgx.NamedArgs{"batch_uuid": batchUUID})
	if err != nil {
		return fmt.Errorf("updating batch %s error: %w", batchUUID, err)
	}

	return p.persistExternalMessage(ctx, walletHash, batchUUID, info)
}

func (p *Postgres) persistExternalMessage(ctx context.Context, walletHash string,
	batchUUID uuid.UUID, info *types.ExtMsgInfo) error {
	p.log.Debug(
		"Persisting ext message",
		"wallet", walletHash,
		"batch", batchUUID,
		"info", *info,
	)

	stmt := `
	INSERT INTO sender.external_message
		(uuid, wallet_hash, batch_uuid, query_id, expired_at, created_at)
	VALUES
		(@uuid, @wallet_hash, @batch_uuid, @query_id, @expired_at, @created_at)
	`

	_, err := p.pg.Exec(ctx, stmt, pgx.NamedArgs{
		"uuid":        info.UUID,
		"wallet_hash": walletHash,
		"batch_uuid":  batchUUID,
		"query_id":    info.QueryID,
		"expired_at":  info.ExpiredAt,
		"created_at":  info.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf(
			"inserting ext message %v error: %w", info, err)
	}

	return nil
}

// ResubmitExpiredBatches finds batches in the processing status and their most
// recent external message that have already expired and re-publishes such
// batches.
func (p *Postgres) ResubmitExpiredBatches(ctx context.Context,
	messageTTL time.Duration) ([]uuid.UUID, error) {
	p.log.Debug("Resubmitting expired batches", "ttl", messageTTL)

	var expiredBatches []uuid.UUID

	stmt := fmt.Sprintf(`
	WITH pending_batches AS (
		SELECT uuid
		FROM sender.batch
		WHERE status = 'processing'
	), expired_batches AS (
		SELECT b.*
		FROM pending_batches b
		LEFT JOIN LATERAL (
			SELECT *
			FROM sender.external_message m
			WHERE b.uuid = m.batch_uuid
			ORDER BY expired_at
			DESC LIMIT 1
		) m ON true
		WHERE m.expired_at < NOW() - INTERVAL '%d seconds'
	)
	SELECT * FROM expired_batches;
	`, uint64(messageTTL.Seconds()))
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
		expiredBatches = append(expiredBatches, id)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return expiredBatches, nil
}
