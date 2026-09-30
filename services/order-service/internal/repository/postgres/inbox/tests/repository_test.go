//go:build integration

package inboxRepository_tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	inboxRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/inbox"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *testutils.TestDatabase

func TestMain(m *testing.M) {
	ctx := context.Background()

	db, err := testutils.SetupTestDatabaseShared(ctx)
	if err != nil {
		panic(err)
	}
	testDB = db
	defer func() { _ = testDB.Close(ctx) }()

	if err := testutils.RunMigrationsCtx(ctx, testDB.ConnStr, migrations.Files); err != nil {
		panic(err)
	}

	code := m.Run()
	os.Exit(code)
}

func truncateAll(t *testing.T) {
	t.Helper()
	testutils.TruncateTables(t, testDB.Pool, "order_items", "saga_state", "orders", "outbox", "inbox_events")
}

func newInboxEvent(eventID uuid.UUID, correlationID uuid.UUID, eventType string) *domain.InboxEvent {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.InboxEvent{
		ID:            uuid.New(),
		EventID:       eventID,
		CorrelationID: correlationID,
		EventType:     eventType,
		Payload: map[string]any{
			"order_id": uuid.New().String(),
			"test_key": "test_value",
		},
		ProcessedAt: now,
	}
}

func TestPostgresInboxRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := inboxRepository.NewPostgresInboxRepository(testDB.Pool)

	t.Run("success - creates inbox event without transaction", func(t *testing.T) {
		defer truncateAll(t)

		eventID := uuid.New()
		correlationID := uuid.New()
		event := newInboxEvent(eventID, correlationID, "inventory.reserved")

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		var count int
		err = testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE event_id = $1",
			eventID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - creates inbox event with empty payload", func(t *testing.T) {
		defer truncateAll(t)

		eventID := uuid.New()
		event := newInboxEvent(eventID, uuid.New(), "payment.succeeded")
		event.Payload = map[string]any{}

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		var count int
		err = testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE event_id = $1",
			eventID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - creates inbox event with complex payload", func(t *testing.T) {
		defer truncateAll(t)

		eventID := uuid.New()
		event := newInboxEvent(eventID, uuid.New(), "inventory.reserved")
		event.Payload = map[string]any{
			"order_id":       uuid.New().String(),
			"reservation_id": uuid.New().String(),
			"items": []map[string]any{
				{"product_id": "prod-1", "quantity": 2},
				{"product_id": "prod-2", "quantity": 5},
			},
			"metadata": map[string]any{
				"source": "kafka",
				"retry":  0,
			},
		}

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		var count int
		err = testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE event_id = $1",
			eventID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - creates inbox event within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		eventID := uuid.New()
		event := newInboxEvent(eventID, uuid.New(), "payment.failed")

		err = repo.Create(txCtx, event)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		var count int
		err = testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE event_id = $1",
			eventID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - rollback within transaction does not persist event", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		eventID := uuid.New()
		event := newInboxEvent(eventID, uuid.New(), "inventory.reservation-failed")

		require.NoError(t, repo.Create(txCtx, event))
		require.NoError(t, tx.Rollback(ctx))

		var count int
		err = testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE event_id = $1",
			eventID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("error - duplicate event_id (idempotency)", func(t *testing.T) {
		defer truncateAll(t)

		eventID := uuid.New()
		correlationID := uuid.New()

		event1 := newInboxEvent(eventID, correlationID, "inventory.reserved")
		event2 := newInboxEvent(eventID, correlationID, "inventory.reserved")

		require.NoError(t, repo.Create(ctx, event1))

		err := repo.Create(ctx, event2)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)

		var count int
		err = testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE event_id = $1",
			eventID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - same correlation_id but different event_id", func(t *testing.T) {
		defer truncateAll(t)

		correlationID := uuid.New()

		event1 := newInboxEvent(uuid.New(), correlationID, "inventory.reserved")
		event2 := newInboxEvent(uuid.New(), correlationID, "payment.succeeded")

		require.NoError(t, repo.Create(ctx, event1))
		require.NoError(t, repo.Create(ctx, event2))

		var count int
		err := testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE correlation_id = $1",
			correlationID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestPostgresInboxRepository_ConcurrentIdempotency(t *testing.T) {
	ctx := context.Background()
	repo := inboxRepository.NewPostgresInboxRepository(testDB.Pool)

	t.Run("concurrent creates with same event_id - one succeeds", func(t *testing.T) {
		defer truncateAll(t)

		eventID := uuid.New()
		correlationID := uuid.New()

		event1 := newInboxEvent(eventID, correlationID, "inventory.reserved")
		event2 := newInboxEvent(eventID, correlationID, "inventory.reserved")

		errChan := make(chan error, 2)

		go func() {
			errChan <- repo.Create(ctx, event1)
		}()

		go func() {
			errChan <- repo.Create(ctx, event2)
		}()

		err1 := <-errChan
		err2 := <-errChan

		if err1 == nil {
			require.ErrorIs(t, err2, domain.ErrAlreadyExists)
		} else if err2 == nil {
			require.ErrorIs(t, err1, domain.ErrAlreadyExists)
		} else {
			t.Fatal("both creates failed")
		}

		var count int
		err := testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM inbox_events WHERE event_id = $1",
			eventID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}
