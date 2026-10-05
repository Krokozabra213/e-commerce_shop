//go:build integration

package outboxRepo_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	outboxRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/outbox/repository/postgres"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/migrations"
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

func TestPostgresOutboxRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := outboxRepo.NewPostgresOutboxRepository(testDB.Pool)

	t.Run("success - creates outbox event without transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		now := time.Now().UTC().Truncate(time.Microsecond)
		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   uuid.New(),
			EventType:     infrakafka.TopicUserCreated,
			Payload: map[string]interface{}{
				"id":    uuid.New().String(),
				"email": "test@example.com",
				"roles": []string{"ROLE_USER"},
			},
			CreatedAt: now,
		}

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		// Проверяем, что событие создано
		var (
			id            uuid.UUID
			aggregateType string
			aggregateID   uuid.UUID
			eventType     string
			payloadJSON   []byte
			createdAt     time.Time
			publishedAt   *time.Time
		)

		err = testDB.Pool.QueryRow(ctx,
			"SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, published_at FROM outbox WHERE id = $1",
			event.ID,
		).Scan(&id, &aggregateType, &aggregateID, &eventType, &payloadJSON, &createdAt, &publishedAt)

		require.NoError(t, err)
		assert.Equal(t, event.ID, id)
		assert.Equal(t, event.AggregateType, aggregateType)
		assert.Equal(t, event.AggregateID, aggregateID)
		assert.Equal(t, event.EventType, eventType)
		assert.True(t, event.CreatedAt.Equal(createdAt))
		assert.Nil(t, publishedAt)

		// Проверяем payload
		var payload map[string]interface{}
		err = json.Unmarshal(payloadJSON, &payload)
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", payload["email"])
		assert.Contains(t, payload["roles"], "ROLE_USER")
	})

	t.Run("success - creates outbox event within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   uuid.New(),
			EventType:     "user.updated",
			Payload: map[string]interface{}{
				"id":         uuid.New().String(),
				"email":      "updated@example.com",
				"updated_at": time.Now().Format(time.RFC3339),
			},
			CreatedAt: time.Now(),
		}

		err = repo.Create(txCtx, event)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		// Проверяем, что событие создано
		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox WHERE id = $1", event.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - creates multiple events for same aggregate", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		aggregateID := uuid.New()
		now := time.Now()

		events := []*domain.OutboxEvent{
			{
				ID:            uuid.New(),
				AggregateType: "user",
				AggregateID:   aggregateID,
				EventType:     "user.created",
				Payload:       map[string]interface{}{"action": "created"},
				CreatedAt:     now,
			},
			{
				ID:            uuid.New(),
				AggregateType: "user",
				AggregateID:   aggregateID,
				EventType:     "user.email_confirmed",
				Payload:       map[string]interface{}{"action": "email_confirmed"},
				CreatedAt:     now.Add(1 * time.Second),
			},
			{
				ID:            uuid.New(),
				AggregateType: "user",
				AggregateID:   aggregateID,
				EventType:     "user.profile_updated",
				Payload:       map[string]interface{}{"action": "profile_updated"},
				CreatedAt:     now.Add(2 * time.Second),
			},
		}

		for _, event := range events {
			err := repo.Create(ctx, event)
			require.NoError(t, err)
		}

		// Проверяем количество событий для aggregate
		var count int
		err := testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM outbox WHERE aggregate_id = $1",
			aggregateID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Проверяем порядок событий
		rows, err := testDB.Pool.Query(ctx,
			"SELECT event_type FROM outbox WHERE aggregate_id = $1 ORDER BY created_at",
			aggregateID,
		)
		require.NoError(t, err)
		defer rows.Close()

		var eventTypes []string
		for rows.Next() {
			var eventType string
			err := rows.Scan(&eventType)
			require.NoError(t, err)
			eventTypes = append(eventTypes, eventType)
		}

		assert.Equal(t, []string{"user.created", "user.email_confirmed", "user.profile_updated"}, eventTypes)
	})

	t.Run("success - creates event with complex nested payload", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   uuid.New(),
			EventType:     "user.created",
			Payload: map[string]interface{}{
				"id":    uuid.New().String(),
				"email": "complex@example.com",
				"roles": []string{"ROLE_USER", "ROLE_ADMIN"},
				"metadata": map[string]interface{}{
					"source":   "oauth",
					"provider": "google",
					"ip":       "127.0.0.1",
					"settings": map[string]interface{}{
						"theme":    "dark",
						"language": "en",
					},
				},
				"created_at": time.Now().Format(time.RFC3339),
			},
			CreatedAt: time.Now(),
		}

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		// Проверяем сохраненный payload
		var payloadJSON []byte
		err = testDB.Pool.QueryRow(ctx,
			"SELECT payload FROM outbox WHERE id = $1",
			event.ID,
		).Scan(&payloadJSON)
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(payloadJSON, &payload)
		require.NoError(t, err)

		// Проверяем вложенные структуры
		assert.Equal(t, "complex@example.com", payload["email"])
		assert.Contains(t, payload["roles"], "ROLE_ADMIN")

		metadata := payload["metadata"].(map[string]interface{})
		assert.Equal(t, "google", metadata["provider"])

		settings := metadata["settings"].(map[string]interface{})
		assert.Equal(t, "dark", settings["theme"])
	})

	t.Run("success - creates events with different aggregate types", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		events := []*domain.OutboxEvent{
			{
				ID:            uuid.New(),
				AggregateType: "user",
				AggregateID:   uuid.New(),
				EventType:     "user.created",
				Payload:       map[string]interface{}{"type": "user"},
				CreatedAt:     time.Now(),
			},
			{
				ID:            uuid.New(),
				AggregateType: "order",
				AggregateID:   uuid.New(),
				EventType:     "order.created",
				Payload:       map[string]interface{}{"type": "order"},
				CreatedAt:     time.Now(),
			},
			{
				ID:            uuid.New(),
				AggregateType: "product",
				AggregateID:   uuid.New(),
				EventType:     "product.created",
				Payload:       map[string]interface{}{"type": "product"},
				CreatedAt:     time.Now(),
			},
		}

		for _, event := range events {
			err := repo.Create(ctx, event)
			require.NoError(t, err)
		}

		// Проверяем количество типов агрегатов
		rows, err := testDB.Pool.Query(ctx,
			"SELECT DISTINCT aggregate_type FROM outbox ORDER BY aggregate_type",
		)
		require.NoError(t, err)
		defer rows.Close()

		var aggregateTypes []string
		for rows.Next() {
			var aggregateType string
			err := rows.Scan(&aggregateType)
			require.NoError(t, err)
			aggregateTypes = append(aggregateTypes, aggregateType)
		}

		assert.Equal(t, []string{"order", "product", "user"}, aggregateTypes)
	})

	t.Run("rollback on transaction error", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   uuid.New(),
			EventType:     "user.created",
			Payload:       map[string]interface{}{"test": "rollback"},
			CreatedAt:     time.Now(),
		}

		err = repo.Create(txCtx, event)
		require.NoError(t, err)

		// Откатываем транзакцию
		err = tx.Rollback(ctx)
		require.NoError(t, err)

		// Проверяем, что событие не создано
		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox WHERE id = $1", event.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("success - creates event with empty payload", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   uuid.New(),
			EventType:     "user.deleted",
			Payload:       map[string]interface{}{},
			CreatedAt:     time.Now(),
		}

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		var payloadJSON []byte
		err = testDB.Pool.QueryRow(ctx,
			"SELECT payload FROM outbox WHERE id = $1",
			event.ID,
		).Scan(&payloadJSON)
		require.NoError(t, err)
		assert.Equal(t, "{}", string(payloadJSON))
	})

	t.Run("success - published_at is null by default", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   uuid.New(),
			EventType:     "user.created",
			Payload:       map[string]interface{}{"test": "published_at"},
			CreatedAt:     time.Now(),
		}

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		// Проверяем, что published_at равен NULL
		var publishedAt *time.Time
		err = testDB.Pool.QueryRow(ctx,
			"SELECT published_at FROM outbox WHERE id = $1",
			event.ID,
		).Scan(&publishedAt)
		require.NoError(t, err)
		assert.Nil(t, publishedAt)
	})

	t.Run("success - preserves created_at timestamp", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "outbox")

		specificTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   uuid.New(),
			EventType:     "user.created",
			Payload:       map[string]interface{}{"test": "timestamp"},
			CreatedAt:     specificTime,
		}

		err := repo.Create(ctx, event)
		require.NoError(t, err)

		var createdAt time.Time
		err = testDB.Pool.QueryRow(ctx,
			"SELECT created_at FROM outbox WHERE id = $1",
			event.ID,
		).Scan(&createdAt)
		require.NoError(t, err)
		assert.True(t, specificTime.Equal(createdAt))
	})
}
