package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/openbuilders/batch-sender/internal/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	DuplicateKeyValue string = "23505"
)

var (
	ErrDuplicateKeyValue = errors.New("duplicate key value")
)

func (p *Postgres) PersistRawTXBatch(txs []types.Transaction) error {
	p.log.Info("Persisting raw batch")
	return nil
}

func (p *Postgres) PersistTX(ctx context.Context, tx types.Transaction) (
	[]types.TXTransfer, error) {

	fields := []string{"transaction_id", "wallet", "amount", "comment"}
	rows := make([][]any, len(tx.Data))

	transfers := make([]types.TXTransfer, len(tx.Data))
	for i, tr := range tx.Data {
		transfers[i] = types.TXTransfer{
			Transfer:      tr,
			TransactionID: tx.ID,
		}
		rows[i] = []any{tx.ID, tr.Wallet, tr.Amount, tr.Comment}
	}

	p.log.Debug("COPY", "fields", fields, "rows", rows)
	inserted, err := p.pg.CopyFrom(ctx, pgx.Identifier{"transfer"}, fields,
		pgx.CopyFromRows(rows))
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			// if for some reason
			if pgErr.Code == DuplicateKeyValue {
				return nil, ErrDuplicateKeyValue
			}
		}
		return nil, fmt.Errorf("couldn't persist tx: %w", err)
	}

	if inserted == 0 {
		return nil, fmt.Errorf("persist tx: %v, no rows inserted", tx)
	}

	return transfers, nil
}
