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
	inserted, err := p.pg.CopyFrom(ctx, pgx.Identifier{"transaction"}, fields,
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
	WITH batched_transactions AS (
		SELECT
			id, order_id, wallet, amount, "comment"
		FROM
			transaction
		WHERE
			status = 'new'
		ORDER BY updated_at
		LIMIT @max_batch_size
	), updated AS (
		UPDATE
			transaction t
		SET
			status = 'batched', updated_at = NOW()
		FROM batched_transactions b
		WHERE
			t.order_id = b.order_id AND
			t.wallet = b.wallet AND
			t.amount = b.amount AND
			t.comment = b.comment
		RETURNING t.id
	), batch_tx_ids AS (
		SELECT @batch_uuid::uuid as uuid, ARRAY_AGG(id) as transaction_ids
		FROM updated
	), batch AS (
		INSERT INTO batch (uuid, transaction_ids)
		SELECT uuid, transaction_ids
		FROM batch_tx_ids
		WHERE EXISTS (SELECT 1 FROM updated)
		RETURNING uuid
	)
	SELECT * FROM batch
	`

	generatedUUID := uuid.New()

	err := p.pg.QueryRow(ctx, stmt, pgx.NamedArgs{
		"max_batch_size": maxItems,
		"batch_uuid": generatedUUID.String(),
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
