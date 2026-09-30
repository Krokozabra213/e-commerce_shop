package main

import (
	"context"
	"fmt"
	"log"
	"time"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TestProducer struct {
	client         *kgo.Client
	schemaRegistry *infrakafka.SchemaRegistryClient // Добавили клиент Schema Registry
}

func NewTestProducer(brokers []string, registryURL string) (*TestProducer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping kafka: %w", err)
	}

	// Инициализируем клиент Schema Registry для тестов
	// Для тестов логгер можно передать nil или дефолтный
	srClient, err := infrakafka.NewSchemaRegistryClient(registryURL)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("init schema registry: %w", err)
	}

	return &TestProducer{
		client:         client,
		schemaRegistry: srClient,
	}, nil
}

func (p *TestProducer) SendOrderCreated(ctx context.Context, topic string, event *eventsv1.OrderCreatedEvent) error {
	// 1. Регистрируем схему в Schema Registry
	schema, err := p.schemaRegistry.RegisterOrGetSchema(topic, eventsv1.EventsProtoSchema)
	if err != nil {
		return fmt.Errorf("register schema: %w", err)
	}

	// 2. Сериализуем сообщение в Confluent Wire Format (с Magic Byte и Schema ID)
	data, err := p.schemaRegistry.Serialize(ctx, schema.ID(), event)
	if err != nil {
		return fmt.Errorf("serialize with schema registry: %w", err)
	}

	var orderID string
	var correlationID string

	if event.Order != nil {
		orderID = event.Order.Id
	}

	if event.Metadata != nil {
		correlationID = event.Metadata.CorrelationId
	}

	key := orderID
	if key == "" {
		key = "invalid-order-" + uuid.New().String()
	}

	headers := []kgo.RecordHeader{
		{Key: "event_type", Value: []byte("order.created")},
	}

	if correlationID != "" {
		headers = append(headers, kgo.RecordHeader{
			Key:   "correlation_id",
			Value: []byte(correlationID),
		})
	}

	record := &kgo.Record{
		Topic:   topic,
		Key:     []byte(key),
		Value:   data,
		Headers: headers,
	}

	return p.client.ProduceSync(ctx, record).FirstErr()
}

func (p *TestProducer) SendOrderCancelled(ctx context.Context, topic string, event *eventsv1.OrderCancelledEvent) error {
	// 1. Регистрируем схему в Schema Registry
	schema, err := p.schemaRegistry.RegisterOrGetSchema(topic, eventsv1.EventsProtoSchema)
	if err != nil {
		return fmt.Errorf("register schema: %w", err)
	}

	// 2. Сериализуем сообщение
	data, err := p.schemaRegistry.Serialize(ctx, schema.ID(), event)
	if err != nil {
		return fmt.Errorf("serialize with schema registry: %w", err)
	}

	var orderID string
	var correlationID string

	if event.Cancellation != nil {
		orderID = event.Cancellation.OrderId
	}

	if event.Metadata != nil {
		correlationID = event.Metadata.CorrelationId
	}

	key := orderID
	if key == "" {
		key = "invalid-order-" + uuid.New().String()
	}

	headers := []kgo.RecordHeader{
		{Key: "event_type", Value: []byte("order.cancel-inventory")},
	}

	if correlationID != "" {
		headers = append(headers, kgo.RecordHeader{
			Key:   "correlation_id",
			Value: []byte(correlationID),
		})
	}

	record := &kgo.Record{
		Topic:   topic,
		Key:     []byte(key),
		Value:   data,
		Headers: headers,
	}

	return p.client.ProduceSync(ctx, record).FirstErr()
}

func (p *TestProducer) SendInvalidMessage(ctx context.Context, topic string, key string, value []byte) error {
	// Этот метод отправляет СЫРЫЕ байты (без Schema Registry).
	// Он идеально подходит для Теста 5, чтобы проверить, как консьюмер отвергает сообщения без Magic Byte.
	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}

	return p.client.ProduceSync(ctx, record).FirstErr()
}

func (p *TestProducer) Close() {
	p.client.Close()
}

func main() {
	brokers := []string{"localhost:9092"}
	schemaRegistryURL := "http://localhost:8081" // URL для локального Schema Registry

	producer, err := NewTestProducer(brokers, schemaRegistryURL)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer producer.Close()

	ctx := context.Background()

	log.Println("=== Starting Kafka event producer tests with Schema Registry ===")

	// Тест 1: Отправка валидного order.created
	log.Println("\n--- Test 1: Valid order.created event ---")
	if err := sendValidOrderCreated(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success")
	}

	time.Sleep(2 * time.Second)

	// Тест 2: Отправка order.created с дубликатами товаров
	log.Println("\n--- Test 2: order.created with duplicate products ---")
	if err := sendOrderCreatedWithDuplicates(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success")
	}

	time.Sleep(2 * time.Second)

	// Тест 3: Отправка order.created с большим количеством товаров
	log.Println("\n--- Test 3: order.created with many items ---")
	if err := sendOrderCreatedWithManyItems(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success")
	}

	time.Sleep(2 * time.Second)

	// Тест 4: Отправка валидного order.cancel-inventory
	log.Println("\n--- Test 4: Valid order.cancel-inventory event ---")
	orderID := uuid.New().String()
	if err := sendValidOrderCancelled(ctx, producer, orderID); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success")
	}

	time.Sleep(2 * time.Second)

	// Тест 5: Битое сообщение в order.created (должно попасть в DLQ)
	log.Println("\n--- Test 5: Invalid proto message to order.created (should go to DLQ) ---")
	if err := sendInvalidProtoToOrderCreated(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 6: order.created с nil metadata (должно попасть в DLQ)
	log.Println("\n--- Test 6: order.created with nil metadata (should go to DLQ) ---")
	if err := sendOrderCreatedWithNilMetadata(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 7: order.created с пустыми items (должно попасть в DLQ)
	log.Println("\n--- Test 7: order.created with empty items (should go to DLQ) ---")
	if err := sendOrderCreatedWithEmptyItems(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 8: order.created с невалидным UUID (должно попасть в DLQ)
	log.Println("\n--- Test 8: order.created with invalid order_id (should go to DLQ) ---")
	if err := sendOrderCreatedWithInvalidUUID(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 9: order.created с нулевым/отрицательным quantity (должно попасть в DLQ)
	log.Println("\n--- Test 9: order.created with invalid quantity (should go to DLQ) ---")
	if err := sendOrderCreatedWithInvalidQuantity(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 10: Несколько валидных событий подряд
	log.Println("\n--- Test 10: Multiple valid events ---")
	for i := 0; i < 5; i++ {
		if err := sendValidOrderCreated(ctx, producer); err != nil {
			log.Printf("❌ Event %d failed: %v", i+1, err)
		} else {
			log.Printf("✅ Event %d sent", i+1)
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("\n--- Test 11: order.created with nil order (should go to DLQ) ---")
	if err := sendOrderCreatedWithNilOrder(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 12: order.created с пустым product_id (должно попасть в DLQ)
	log.Println("\n--- Test 12: order.created with empty product_id (should go to DLQ) ---")
	if err := sendOrderCreatedWithEmptyProductID(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 13: order.cancel-inventory с nil cancellation (должно попасть в DLQ)
	log.Println("\n--- Test 13: order.cancel-inventory with nil cancellation (should go to DLQ) ---")
	if err := sendOrderCancelledWithNilCancellation(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	time.Sleep(2 * time.Second)

	// Тест 14: order.cancel-inventory с пустым order_id (должно попасть в DLQ)
	log.Println("\n--- Test 14: order.cancel-inventory with empty order_id (should go to DLQ) ---")
	if err := sendOrderCancelledWithEmptyOrderID(ctx, producer); err != nil {
		log.Printf("❌ Failed: %v", err)
	} else {
		log.Println("✅ Success - check DLQ topic")
	}

	log.Println("\n=== All tests completed ===")
	log.Println("\nTo check DLQ messages, run:")
	log.Println("  kafka-console-consumer --bootstrap-server localhost:9092 --topic inventory-service.dlq --from-beginning --property print.headers=true")

	log.Println("\n=== All tests completed ===")
}

// Тест 1: Валидное событие order.created
func sendValidOrderCreated(ctx context.Context, producer *TestProducer) error {
	orderID := uuid.New()
	correlationID := uuid.New()

	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: correlationID.String(),
		},
		Order: &eventsv1.OrderData{
			Id: orderID.String(),
			Items: []*eventsv1.ProductItem{
				{ProductId: uuid.New().String(), Quantity: 5},
				{ProductId: uuid.New().String(), Quantity: 10},
				{ProductId: uuid.New().String(), Quantity: 3},
			},
			CreatedAt: timestamppb.Now(),
		},
	}

	log.Printf("Sending order.created: order_id=%s, correlation_id=%s, items=%d",
		orderID, correlationID, len(event.Order.Items))

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 2: order.created с дубликатами товаров
func sendOrderCreatedWithDuplicates(ctx context.Context, producer *TestProducer) error {
	orderID := uuid.New()
	correlationID := uuid.New()
	productID := uuid.New().String()

	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: correlationID.String(),
		},
		Order: &eventsv1.OrderData{
			Id: orderID.String(),
			Items: []*eventsv1.ProductItem{
				{ProductId: productID, Quantity: 5},
				{ProductId: productID, Quantity: 3}, // Дубликат
				{ProductId: uuid.New().String(), Quantity: 10},
			},
			CreatedAt: timestamppb.Now(),
		},
	}

	log.Printf("Sending order.created with duplicates: order_id=%s, duplicate_product=%s",
		orderID, productID)

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 3: order.created с большим количеством товаров
func sendOrderCreatedWithManyItems(ctx context.Context, producer *TestProducer) error {
	orderID := uuid.New()
	correlationID := uuid.New()

	items := make([]*eventsv1.ProductItem, 50)
	for i := 0; i < 50; i++ {
		items[i] = &eventsv1.ProductItem{
			ProductId: uuid.New().String(),
			Quantity:  int32(i + 1),
		}
	}

	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: correlationID.String(),
		},
		Order: &eventsv1.OrderData{
			Id:        orderID.String(),
			Items:     items,
			CreatedAt: timestamppb.Now(),
		},
	}

	log.Printf("Sending order.created with many items: order_id=%s, items_count=%d",
		orderID, len(items))

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 4: Валидное событие order.cancel-inventory
func sendValidOrderCancelled(ctx context.Context, producer *TestProducer, orderID string) error {
	correlationID := uuid.New()

	event := &eventsv1.OrderCancelledEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.cancel-inventory",
			Timestamp:     timestamppb.Now(),
			CorrelationId: correlationID.String(),
		},
		Cancellation: &eventsv1.OrderCancellationData{
			OrderId:     orderID,
			Reason:      "customer request",
			CancelledAt: timestamppb.Now(),
		},
	}

	log.Printf("Sending order.cancel-inventory: order_id=%s, reason=%s",
		orderID, event.Cancellation.Reason)

	return producer.SendOrderCancelled(ctx, "order.cancel-inventory", event)
}

// Тест 5: Невалидное protobuf сообщение
func sendInvalidProtoToOrderCreated(ctx context.Context, producer *TestProducer) error {
	invalidData := []byte("this is not a valid protobuf message")

	log.Printf("Sending invalid proto to order.created (size=%d bytes)", len(invalidData))

	return producer.SendInvalidMessage(ctx, "order.created", "invalid-key", invalidData)
}

// Тест 6: order.created с nil metadata
func sendOrderCreatedWithNilMetadata(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCreatedEvent{
		Metadata: nil, // Nil metadata
		Order: &eventsv1.OrderData{
			Id: uuid.New().String(),
			Items: []*eventsv1.ProductItem{
				{ProductId: uuid.New().String(), Quantity: 5},
			},
		},
	}

	log.Println("Sending order.created with nil metadata")

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 7: order.created с пустым списком items
func sendOrderCreatedWithEmptyItems(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: uuid.New().String(),
		},
		Order: &eventsv1.OrderData{
			Id:        uuid.New().String(),
			Items:     []*eventsv1.ProductItem{}, // Пустой список
			CreatedAt: timestamppb.Now(),
		},
	}

	log.Println("Sending order.created with empty items")

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 8: order.created с невалидным UUID
func sendOrderCreatedWithInvalidUUID(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: "not-a-valid-uuid", // Невалидный UUID
		},
		Order: &eventsv1.OrderData{
			Id: "also-not-a-uuid", // Невалидный UUID
			Items: []*eventsv1.ProductItem{
				{ProductId: uuid.New().String(), Quantity: 5},
			},
			CreatedAt: timestamppb.Now(),
		},
	}

	log.Println("Sending order.created with invalid UUIDs")

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 9: order.created с невалидным quantity
func sendOrderCreatedWithInvalidQuantity(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: uuid.New().String(),
		},
		Order: &eventsv1.OrderData{
			Id: uuid.New().String(),
			Items: []*eventsv1.ProductItem{
				{ProductId: uuid.New().String(), Quantity: 0},  // Нулевое количество
				{ProductId: uuid.New().String(), Quantity: -5}, // Отрицательное количество
			},
			CreatedAt: timestamppb.Now(),
		},
	}

	log.Println("Sending order.created with invalid quantities (0 and -5)")

	return producer.SendOrderCreated(ctx, "order.created", event)
}

func sendOrderCreatedWithNilOrder(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: uuid.New().String(),
		},
		Order: nil, // Nil order
	}

	log.Println("Sending order.created with nil order")

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 12: order.created с пустым product_id
func sendOrderCreatedWithEmptyProductID(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCreatedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.created",
			Timestamp:     timestamppb.Now(),
			CorrelationId: uuid.New().String(),
		},
		Order: &eventsv1.OrderData{
			Id: uuid.New().String(),
			Items: []*eventsv1.ProductItem{
				{ProductId: "", Quantity: 5}, // Пустой product_id
			},
			CreatedAt: timestamppb.Now(),
		},
	}

	log.Println("Sending order.created with empty product_id")

	return producer.SendOrderCreated(ctx, "order.created", event)
}

// Тест 13: order.cancel-inventory с nil cancellation
func sendOrderCancelledWithNilCancellation(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCancelledEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.cancel-inventory",
			Timestamp:     timestamppb.Now(),
			CorrelationId: uuid.New().String(),
		},
		Cancellation: nil, // Nil cancellation
	}

	log.Println("Sending order.cancel-inventory with nil cancellation")

	return producer.SendOrderCancelled(ctx, "order.cancel-inventory", event)
}

// Тест 14: order.cancel-inventory с пустым order_id
func sendOrderCancelledWithEmptyOrderID(ctx context.Context, producer *TestProducer) error {
	event := &eventsv1.OrderCancelledEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       uuid.New().String(),
			EventType:     "order.cancel-inventory",
			Timestamp:     timestamppb.Now(),
			CorrelationId: uuid.New().String(),
		},
		Cancellation: &eventsv1.OrderCancellationData{
			OrderId:     "", // Пустой order_id
			Reason:      "test",
			CancelledAt: timestamppb.Now(),
		},
	}

	log.Println("Sending order.cancel-inventory with empty order_id")

	return producer.SendOrderCancelled(ctx, "order.cancel-inventory", event)
}
