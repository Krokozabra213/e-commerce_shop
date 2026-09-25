//go:build integration

package outboxRepository_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	outboxRepository "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/repository/postgres/outbox"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/migrations"
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
	testutils.TruncateTables(t, testDB.Pool, "stocks", "outbox", "reservations")
}

func newOutboxEvent(aggregateType, eventType string, payload map[string]interface{}) *domain.OutboxEvent {
	now := time.Now().UTC().Truncate(time.Microsecond)
	payloadBytes, _ := json.Marshal(payload)

	return &domain.OutboxEvent{
		ID:            uuid.New(),
		CorrelationID: uuid.New(),
		AggregateType: aggregateType,
		AggregateID:   uuid.New(),
		EventType:     eventType,
		Payload:       payloadBytes,
		CreatedAt:     now,
	}
}

func createTestOutboxEvent(t *testing.T, ctx context.Context, repo *outboxRepository.PostgresOutboxRepository, event *domain.OutboxEvent) {
	t.Helper()
	require.NoError(t, repo.Create(ctx, event))
}

func TestPostgresOutboxRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := outboxRepository.NewPostgresOutboxRepository(testDB.Pool)

	t.Run("success - creates outbox event without transaction", func(t *testing.T) {
		defer truncateAll(t)

		payload := map[string]interface{}{
			"product_id": "test-product",
			"quantity":   10,
		}
		event := newOutboxEvent("Stock", "StockReserved", payload)

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 1)

		found := events[0]
		assert.Equal(t, event.ID, found.ID)
		assert.Equal(t, event.CorrelationID, found.CorrelationID)
		assert.Equal(t, event.AggregateType, found.AggregateType)
		assert.Equal(t, event.AggregateID, found.AggregateID)
		assert.Equal(t, event.EventType, found.EventType)
		assert.JSONEq(t, string(event.Payload), string(found.Payload))
		assert.WithinDuration(t, event.CreatedAt, found.CreatedAt, time.Second)
		assert.Nil(t, found.PublishedAt)
	})

	t.Run("success - creates event with complex payload", func(t *testing.T) {
		defer truncateAll(t)

		payload := map[string]interface{}{
			"order_id": "order-123",
			"items": []map[string]interface{}{
				{"product_id": "p1", "quantity": 5},
				{"product_id": "p2", "quantity": 10},
			},
			"metadata": map[string]string{
				"user_id": "user-456",
				"source":  "web",
			},
		}
		event := newOutboxEvent("Order", "OrderCreated", payload)

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.JSONEq(t, string(event.Payload), string(events[0].Payload))
	})

	t.Run("success - creates event within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		payload := map[string]interface{}{"test": "data"}
		event := newOutboxEvent("Stock", "StockUpdated", payload)

		err = repo.Create(txCtx, event)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, event.ID, events[0].ID)
	})

	t.Run("success - rollback within transaction does not persist event", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		payload := map[string]interface{}{"test": "data"}
		event := newOutboxEvent("Stock", "StockUpdated", payload)

		require.NoError(t, repo.Create(txCtx, event))
		require.NoError(t, tx.Rollback(ctx))

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Empty(t, events)
	})

	t.Run("error - duplicate id", func(t *testing.T) {
		defer truncateAll(t)

		eventID := uuid.New()
		payload1 := map[string]interface{}{"key": "value1"}
		payload2 := map[string]interface{}{"key": "value2"}

		event1 := newOutboxEvent("Stock", "Event1", payload1)
		event1.ID = eventID

		event2 := newOutboxEvent("Stock", "Event2", payload2)
		event2.ID = eventID

		require.NoError(t, repo.Create(ctx, event1))

		err := repo.Create(ctx, event2)
		require.ErrorIs(t, err, domain.AlreadyExistsError)
	})

	t.Run("success - creates multiple events", func(t *testing.T) {
		defer truncateAll(t)

		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", "StockEvent", payload)
			require.NoError(t, repo.Create(ctx, event))
		}

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, events, 5)
	})
}

func TestPostgresOutboxRepository_FetchUnpublished(t *testing.T) {
	ctx := context.Background()
	repo := outboxRepository.NewPostgresOutboxRepository(testDB.Pool)

	t.Run("success - fetches unpublished events", func(t *testing.T) {
		defer truncateAll(t)

		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", "StockEvent", payload)
			createTestOutboxEvent(t, ctx, repo, event)
			time.Sleep(time.Millisecond)
		}

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 5)
	})

	t.Run("success - respects limit", func(t *testing.T) {
		defer truncateAll(t)

		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", "StockEvent", payload)
			createTestOutboxEvent(t, ctx, repo, event)
		}

		events, err := repo.FetchUnpublished(ctx, 3)
		require.NoError(t, err)
		require.Len(t, events, 3)
	})

	t.Run("success - orders by created_at ASC", func(t *testing.T) {
		defer truncateAll(t)

		var createdEvents []*domain.OutboxEvent
		for i := 0; i < 3; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", "StockEvent", payload)
			createTestOutboxEvent(t, ctx, repo, event)
			createdEvents = append(createdEvents, event)
			time.Sleep(5 * time.Millisecond)
		}

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 3)

		for i := 0; i < 3; i++ {
			assert.Equal(t, createdEvents[i].ID, events[i].ID)
		}
	})

	t.Run("success - does not fetch published events", func(t *testing.T) {
		defer truncateAll(t)

		event1 := newOutboxEvent("Stock", "Event1", map[string]interface{}{"n": 1})
		event2 := newOutboxEvent("Stock", "Event2", map[string]interface{}{"n": 2})
		event3 := newOutboxEvent("Stock", "Event3", map[string]interface{}{"n": 3})

		createTestOutboxEvent(t, ctx, repo, event1)
		createTestOutboxEvent(t, ctx, repo, event2)
		createTestOutboxEvent(t, ctx, repo, event3)

		require.NoError(t, repo.MarkPublished(ctx, event2.ID))

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 2)

		ids := []uuid.UUID{events[0].ID, events[1].ID}
		assert.Contains(t, ids, event1.ID)
		assert.Contains(t, ids, event3.ID)
		assert.NotContains(t, ids, event2.ID)
	})

	t.Run("success - returns empty slice when no unpublished events", func(t *testing.T) {
		defer truncateAll(t)

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Empty(t, events)
	})

	t.Run("success - fetches within transaction", func(t *testing.T) {
		defer truncateAll(t)

		event := newOutboxEvent("Stock", "Event", map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		events, err := repo.FetchUnpublished(txCtx, 10)
		require.NoError(t, err)
		require.Len(t, events, 1)
	})

	t.Run("success - FOR UPDATE SKIP LOCKED prevents concurrent fetching", func(t *testing.T) {
		defer truncateAll(t)

		event := newOutboxEvent("Stock", "Event", map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		tx1, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback(ctx) }()

		tx1Ctx := tx_manager.CtxWithTx(ctx, tx1)

		events1, err := repo.FetchUnpublished(tx1Ctx, 10)
		require.NoError(t, err)
		require.Len(t, events1, 1)

		tx2, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback(ctx) }()

		tx2Ctx := tx_manager.CtxWithTx(ctx, tx2)

		events2, err := repo.FetchUnpublished(tx2Ctx, 10)
		require.NoError(t, err)
		require.Empty(t, events2)
	})
}

func TestPostgresOutboxRepository_MarkPublished(t *testing.T) {
	ctx := context.Background()
	repo := outboxRepository.NewPostgresOutboxRepository(testDB.Pool)

	t.Run("success - marks event as published", func(t *testing.T) {
		defer truncateAll(t)

		event := newOutboxEvent("Stock", "Event", map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		beforePublish := time.Now().UTC()
		time.Sleep(10 * time.Millisecond)

		err := repo.MarkPublished(ctx, event.ID)
		require.NoError(t, err)

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Empty(t, events)

		var publishedAt *time.Time
		query := `SELECT published_at FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&publishedAt)
		require.NoError(t, err)
		require.NotNil(t, publishedAt)
		assert.True(t, publishedAt.After(beforePublish))
	})

	t.Run("success - marks within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		event := newOutboxEvent("Stock", "Event", map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		err = repo.MarkPublished(txCtx, event.ID)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Empty(t, events)
	})

	t.Run("success - rollback within transaction does not mark as published", func(t *testing.T) {
		defer truncateAll(t)

		event := newOutboxEvent("Stock", "Event", map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.MarkPublished(txCtx, event.ID))
		require.NoError(t, tx.Rollback(ctx))

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, event.ID, events[0].ID)
	})

	t.Run("error - event not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentID := uuid.New()

		err := repo.MarkPublished(ctx, nonExistentID)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("success - marking already published event updates published_at", func(t *testing.T) {
		defer truncateAll(t)

		event := newOutboxEvent("Stock", "Event", map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		err := repo.MarkPublished(ctx, event.ID)
		require.NoError(t, err)

		var firstPublishedAt time.Time
		query := `SELECT published_at FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&firstPublishedAt)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		err = repo.MarkPublished(ctx, event.ID)
		require.NoError(t, err)

		var secondPublishedAt time.Time
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&secondPublishedAt)
		require.NoError(t, err)

		assert.True(t, secondPublishedAt.After(firstPublishedAt))
	})

	t.Run("success - marks only specified event", func(t *testing.T) {
		defer truncateAll(t)

		event1 := newOutboxEvent("Stock", "Event1", map[string]interface{}{"n": 1})
		event2 := newOutboxEvent("Stock", "Event2", map[string]interface{}{"n": 2})
		event3 := newOutboxEvent("Stock", "Event3", map[string]interface{}{"n": 3})

		createTestOutboxEvent(t, ctx, repo, event1)
		createTestOutboxEvent(t, ctx, repo, event2)
		createTestOutboxEvent(t, ctx, repo, event3)

		err := repo.MarkPublished(ctx, event2.ID)
		require.NoError(t, err)

		events, err := repo.FetchUnpublished(ctx, 10)
		require.NoError(t, err)
		require.Len(t, events, 2)

		ids := []uuid.UUID{events[0].ID, events[1].ID}
		assert.Contains(t, ids, event1.ID)
		assert.Contains(t, ids, event3.ID)
		assert.NotContains(t, ids, event2.ID)
	})

	t.Run("success - published_at is set to current timestamp", func(t *testing.T) {
		defer truncateAll(t)

		event := newOutboxEvent("Stock", "Event", map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		before := time.Now().UTC()
		err := repo.MarkPublished(ctx, event.ID)
		after := time.Now().UTC()

		require.NoError(t, err)

		var publishedAt time.Time
		query := `SELECT published_at FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&publishedAt)
		require.NoError(t, err)

		assert.True(t, publishedAt.After(before) || publishedAt.Equal(before))
		assert.True(t, publishedAt.Before(after) || publishedAt.Equal(after))
	})
}
