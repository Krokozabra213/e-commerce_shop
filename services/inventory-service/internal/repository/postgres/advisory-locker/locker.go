package advisory_locker

import (
	"context"
	"fmt"
	"hash/fnv"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdvisoryLocker struct {
	pool *pgxpool.Pool
}

func NewAdvisoryLocker(pool *pgxpool.Pool) *AdvisoryLocker {
	return &AdvisoryLocker{pool: pool}
}

func (l *AdvisoryLocker) LockByUUID(ctx context.Context, id uuid.UUID) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(l.pool, tx)

	key := hashUUID(id)

	_, err := querier.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	if err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}

	return nil
}

func hashUUID(id uuid.UUID) int64 {
	h := fnv.New64a()
	h.Write(id[:])
	return int64(h.Sum64())
}
