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
		event := newOutboxEvent("Stock", "inventory.stock.reserved", payload)

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		events, err := repo.FetchUnpublishedByEventType(ctx, "inventory.stock.reserved", 10, "worker-1", 5*time.Second)
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
		event := newOutboxEvent("Order", "inventory.reservation.created", payload)

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		events, err := repo.FetchUnpublishedByEventType(ctx, "inventory.reservation.created", 10, "worker-1", 5*time.Second)
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
		event := newOutboxEvent("Stock", "inventory.stock.updated", payload)

		err = repo.Create(txCtx, event)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		events, err := repo.FetchUnpublishedByEventType(ctx, "inventory.stock.updated", 10, "worker-1", 5*time.Second)
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
		event := newOutboxEvent("Stock", "inventory.stock.updated", payload)

		require.NoError(t, repo.Create(txCtx, event))
		require.NoError(t, tx.Rollback(ctx))

		events, err := repo.FetchUnpublishedByEventType(ctx, "inventory.stock.updated", 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Empty(t, events)
	})

	t.Run("error - duplicate id", func(t *testing.T) {
		defer truncateAll(t)

		eventID := uuid.New()
		payload1 := map[string]interface{}{"key": "value1"}
		payload2 := map[string]interface{}{"key": "value2"}

		event1 := newOutboxEvent("Stock", "inventory.event1", payload1)
		event1.ID = eventID

		event2 := newOutboxEvent("Stock", "inventory.event2", payload2)
		event2.ID = eventID

		require.NoError(t, repo.Create(ctx, event1))

		err := repo.Create(ctx, event2)
		require.ErrorIs(t, err, domain.AlreadyExistsError)
	})

	t.Run("success - creates multiple events", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.event"
		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", eventType, payload)
			require.NoError(t, repo.Create(ctx, event))
		}

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		assert.Len(t, events, 5)
	})
}

func TestPostgresOutboxRepository_FetchUnpublishedByEventType(t *testing.T) {
	ctx := context.Background()
	repo := outboxRepository.NewPostgresOutboxRepository(testDB.Pool)

	t.Run("success - fetches unpublished events by event type", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		var createdIDs []uuid.UUID

		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", eventType, payload)
			createTestOutboxEvent(t, ctx, repo, event)
			createdIDs = append(createdIDs, event.ID)
		}

		// Создаем события другого типа
		otherEventType := "inventory.stock.released"
		for i := 0; i < 3; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", otherEventType, payload)
			createTestOutboxEvent(t, ctx, repo, event)
		}

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 5)

		// Проверяем, что все события нужного типа и все созданные события присутствуют
		fetchedIDs := make(map[uuid.UUID]bool)
		for _, e := range events {
			assert.Equal(t, eventType, e.EventType)
			fetchedIDs[e.ID] = true
		}

		for _, id := range createdIDs {
			assert.True(t, fetchedIDs[id], "Created event %s should be fetched", id)
		}
	})

	t.Run("success - respects limit", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", eventType, payload)
			createTestOutboxEvent(t, ctx, repo, event)
		}

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 3, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 3)

		// Все события правильного типа
		for _, e := range events {
			assert.Equal(t, eventType, e.EventType)
		}
	})

	t.Run("success - fetches all unpublished events when limit is higher", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		for i := 0; i < 3; i++ {
			payload := map[string]interface{}{"index": i}
			event := newOutboxEvent("Stock", eventType, payload)
			createTestOutboxEvent(t, ctx, repo, event)
		}

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 3)
	})

	t.Run("success - does not fetch published events", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event1 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 1})
		event2 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 2})
		event3 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 3})

		createTestOutboxEvent(t, ctx, repo, event1)
		createTestOutboxEvent(t, ctx, repo, event2)
		createTestOutboxEvent(t, ctx, repo, event3)

		require.NoError(t, repo.MarkPublished(ctx, []uuid.UUID{event2.ID}))

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 2)

		ids := []uuid.UUID{events[0].ID, events[1].ID}
		assert.Contains(t, ids, event1.ID)
		assert.Contains(t, ids, event3.ID)
		assert.NotContains(t, ids, event2.ID)
	})

	t.Run("success - returns empty slice when no unpublished events", func(t *testing.T) {
		defer truncateAll(t)

		events, err := repo.FetchUnpublishedByEventType(ctx, "inventory.stock.reserved", 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Empty(t, events)
	})

	t.Run("success - returns empty slice for non-existent event type", func(t *testing.T) {
		defer truncateAll(t)

		// Создаем события одного типа
		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		// Запрашиваем другой тип
		events, err := repo.FetchUnpublishedByEventType(ctx, "inventory.stock.released", 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Empty(t, events)
	})

	t.Run("success - locks events with lease", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		workerID := "worker-1"
		lease := 5 * time.Second

		before := time.Now().UTC()
		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, workerID, lease)
		after := time.Now().UTC()

		require.NoError(t, err)
		require.Len(t, events, 1)

		// Проверяем, что событие заблокировано в БД
		var lockedBy *string
		var lockedUntil *time.Time
		query := `SELECT locked_by, locked_until FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&lockedBy, &lockedUntil)
		require.NoError(t, err)

		require.NotNil(t, lockedBy)
		assert.Equal(t, workerID, *lockedBy)

		require.NotNil(t, lockedUntil)
		expectedLockTime := before.Add(lease)
		assert.True(t, lockedUntil.After(expectedLockTime) || lockedUntil.Equal(expectedLockTime))
		assert.True(t, lockedUntil.Before(after.Add(lease)) || lockedUntil.Equal(after.Add(lease)))
	})

	t.Run("success - does not fetch locked events", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		// Worker-1 берет событие
		events1, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events1, 1)

		// Worker-2 пытается взять то же событие
		events2, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-2", 5*time.Second)
		require.NoError(t, err)
		require.Empty(t, events2)
	})

	t.Run("success - fetches events after lease expiration", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		// Worker-1 берет событие с коротким lease
		shortLease := 100 * time.Millisecond
		events1, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", shortLease)
		require.NoError(t, err)
		require.Len(t, events1, 1)

		// Ждем истечения lease
		time.Sleep(150 * time.Millisecond)

		// Worker-2 может взять событие после истечения lease
		events2, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-2", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events2, 1)
		assert.Equal(t, event.ID, events2[0].ID)
	})

	t.Run("success - different workers process different event types", func(t *testing.T) {
		defer truncateAll(t)

		eventType1 := "inventory.stock.reserved"
		eventType2 := "inventory.stock.released"

		event1 := newOutboxEvent("Stock", eventType1, map[string]interface{}{"type": 1})
		event2 := newOutboxEvent("Stock", eventType2, map[string]interface{}{"type": 2})

		createTestOutboxEvent(t, ctx, repo, event1)
		createTestOutboxEvent(t, ctx, repo, event2)

		// Worker-1 обрабатывает eventType1
		events1, err := repo.FetchUnpublishedByEventType(ctx, eventType1, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events1, 1)
		assert.Equal(t, event1.ID, events1[0].ID)

		// Worker-2 обрабатывает eventType2
		events2, err := repo.FetchUnpublishedByEventType(ctx, eventType2, 10, "worker-2", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events2, 1)
		assert.Equal(t, event2.ID, events2[0].ID)
	})

	t.Run("success - FOR UPDATE SKIP LOCKED prevents concurrent fetching of same events", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		for i := 0; i < 5; i++ {
			event := newOutboxEvent("Stock", eventType, map[string]interface{}{"index": i})
			createTestOutboxEvent(t, ctx, repo, event)
		}

		tx1, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback(ctx) }()

		tx1Ctx := tx_manager.CtxWithTx(ctx, tx1)

		events1, err := repo.FetchUnpublishedByEventType(tx1Ctx, eventType, 3, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events1, 3)

		tx2, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback(ctx) }()

		tx2Ctx := tx_manager.CtxWithTx(ctx, tx2)

		// Worker-2 должен получить оставшиеся 2 события (не заблокированные Worker-1)
		events2, err := repo.FetchUnpublishedByEventType(tx2Ctx, eventType, 10, "worker-2", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events2, 2)

		// Проверяем, что события не пересекаются
		ids1 := make(map[uuid.UUID]bool)
		for _, e := range events1 {
			ids1[e.ID] = true
		}

		for _, e := range events2 {
			assert.False(t, ids1[e.ID], "Worker-2 не должен получить события, заблокированные Worker-1")
		}
	})

	t.Run("success - fetches within transaction", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		events, err := repo.FetchUnpublishedByEventType(txCtx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, event.ID, events[0].ID)
	})
}

func TestPostgresOutboxRepository_MarkPublished(t *testing.T) {
	ctx := context.Background()
	repo := outboxRepository.NewPostgresOutboxRepository(testDB.Pool)

	t.Run("success - marks event as published", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		beforePublish := time.Now().UTC()
		time.Sleep(10 * time.Millisecond)

		err := repo.MarkPublished(ctx, []uuid.UUID{event.ID})
		require.NoError(t, err)

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Empty(t, events)

		var publishedAt *time.Time
		query := `SELECT published_at FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&publishedAt)
		require.NoError(t, err)
		require.NotNil(t, publishedAt)
		assert.True(t, publishedAt.After(beforePublish))
	})

	t.Run("success - marks multiple events as published", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event1 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 1})
		event2 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 2})
		event3 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 3})

		createTestOutboxEvent(t, ctx, repo, event1)
		createTestOutboxEvent(t, ctx, repo, event2)
		createTestOutboxEvent(t, ctx, repo, event3)

		err := repo.MarkPublished(ctx, []uuid.UUID{event1.ID, event3.ID})
		require.NoError(t, err)

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, event2.ID, events[0].ID)
	})

	t.Run("success - marks within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		err = repo.MarkPublished(txCtx, []uuid.UUID{event.ID})
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Empty(t, events)
	})

	t.Run("success - rollback within transaction does not mark as published", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.MarkPublished(txCtx, []uuid.UUID{event.ID}))
		require.NoError(t, tx.Rollback(ctx))

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, event.ID, events[0].ID)
	})

	t.Run("error - event not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentID := uuid.New()

		err := repo.MarkPublished(ctx, []uuid.UUID{nonExistentID})
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("error - some events not found", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		nonExistentID := uuid.New()

		// Пытаемся опубликовать существующий и несуществующий ID
		err := repo.MarkPublished(ctx, []uuid.UUID{event.ID, nonExistentID})
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("success - marks only specified events", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event1 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 1})
		event2 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 2})
		event3 := newOutboxEvent("Stock", eventType, map[string]interface{}{"n": 3})

		createTestOutboxEvent(t, ctx, repo, event1)
		createTestOutboxEvent(t, ctx, repo, event2)
		createTestOutboxEvent(t, ctx, repo, event3)

		err := repo.MarkPublished(ctx, []uuid.UUID{event2.ID})
		require.NoError(t, err)

		events, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)
		require.Len(t, events, 2)

		ids := []uuid.UUID{events[0].ID, events[1].ID}
		assert.Contains(t, ids, event1.ID)
		assert.Contains(t, ids, event3.ID)
		assert.NotContains(t, ids, event2.ID)
	})

	t.Run("success - published_at is set to current timestamp", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		before := time.Now().UTC()
		err := repo.MarkPublished(ctx, []uuid.UUID{event.ID})
		after := time.Now().UTC()

		require.NoError(t, err)

		var publishedAt time.Time
		query := `SELECT published_at FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&publishedAt)
		require.NoError(t, err)

		assert.True(t, publishedAt.After(before) || publishedAt.Equal(before))
		assert.True(t, publishedAt.Before(after) || publishedAt.Equal(after))
	})

	t.Run("success - clears lock when marking as published", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		// Сначала блокируем событие
		_, err := repo.FetchUnpublishedByEventType(ctx, eventType, 10, "worker-1", 5*time.Second)
		require.NoError(t, err)

		// Проверяем, что событие заблокировано
		var lockedBy *string
		query := `SELECT locked_by FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, query, event.ID).Scan(&lockedBy)
		require.NoError(t, err)
		require.NotNil(t, lockedBy)

		// Публикуем событие
		err = repo.MarkPublished(ctx, []uuid.UUID{event.ID})
		require.NoError(t, err)

		// Проверяем, что блокировка очищена и published_at установлен
		var publishedAt *time.Time
		var lockedByAfter *string
		var lockedUntilAfter *time.Time
		queryFull := `SELECT published_at, locked_by, locked_until FROM outbox WHERE id = $1`
		err = testDB.Pool.QueryRow(ctx, queryFull, event.ID).Scan(&publishedAt, &lockedByAfter, &lockedUntilAfter)
		require.NoError(t, err)
		require.NotNil(t, publishedAt)
		assert.Nil(t, lockedByAfter)
		assert.Nil(t, lockedUntilAfter)
	})

	t.Run("success - handles empty ids slice", func(t *testing.T) {
		defer truncateAll(t)

		err := repo.MarkPublished(ctx, []uuid.UUID{})
		require.NoError(t, err)
	})

	t.Run("error - cannot mark already published event again", func(t *testing.T) {
		defer truncateAll(t)

		eventType := "inventory.stock.reserved"
		event := newOutboxEvent("Stock", eventType, map[string]interface{}{"test": "data"})
		createTestOutboxEvent(t, ctx, repo, event)

		// Публикуем первый раз
		err := repo.MarkPublished(ctx, []uuid.UUID{event.ID})
		require.NoError(t, err)

		// Пытаемся опубликовать второй раз
		err = repo.MarkPublished(ctx, []uuid.UUID{event.ID})
		require.ErrorIs(t, err, domain.NotFoundError)
	})
}
