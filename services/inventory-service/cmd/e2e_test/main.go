package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"time"

	infraConfig "github.com/Krokozabra213/e-commerce_shop/infra/config"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/kafka"
	"github.com/google/uuid"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfgProducer := infraConfig.KafkaProducerConfig{
		Brokers:      []string{"localhost:9092"},
		ClientID:     "inventory-mock-publisher",
		DialTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxRetries:   5,
		RetryBackoff: 300 * time.Millisecond,
	}

	kafkaProducer, err := infrakafka.NewKGOProducer(cfgProducer, slog.Default())
	if err != nil {
		log.Fatalf("Не удалось создать kafka продюсер: %v", err)
	}
	defer kafkaProducer.Close()

	schemaRegistry, err := infrakafka.NewSchemaRegistryClient("http://localhost:8081")
	if err != nil {
		log.Fatalf("Не удалось подключиться к Schema Registry: %v", err)
	}

	publisher := kafka.NewEventPublisher(kafkaProducer, schemaRegistry, slog.Default())

	orderID := uuid.MustParse("0fe63e54-c984-46d5-b21b-b6064edc58d4")
	correlationID := uuid.MustParse("761bb839-3f59-4855-8711-c5754be41d50")

	payloadMap := map[string]any{
		"order_id": orderID.String(),
	}
	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		log.Fatalf("Ошибка маршалинга payload: %v", err)
	}

	mockEvent := &domain.OutboxEvent{
		ID:            uuid.New(),
		EventType:     "inventory.reserved",
		AggregateType: "order",
		AggregateID:   orderID,
		CorrelationID: correlationID,
		Payload:       payloadBytes,
		CreatedAt:     time.Now(),
	}

	err = publisher.Publish(ctx, mockEvent)
	if err != nil {
		log.Fatalf("Ошибка отправки мока: %v", err)
	}

	log.Println("🎉 Имитационное сообщение успешно отправлено в топик inventory.reserved!")
}

func main2() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfgProducer := infraConfig.KafkaProducerConfig{
		Brokers:      []string{"localhost:9092"},
		ClientID:     "inventory-mock-publisher",
		DialTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxRetries:   5,
		RetryBackoff: 300 * time.Millisecond,
	}

	kafkaProducer, err := infrakafka.NewKGOProducer(cfgProducer, slog.Default())
	if err != nil {
		log.Fatalf("Не удалось создать kafka продюсер: %v", err)
	}
	defer kafkaProducer.Close()

	schemaRegistry, err := infrakafka.NewSchemaRegistryClient("http://localhost:8081")
	if err != nil {
		log.Fatalf("Не удалось подключиться к Schema Registry: %v", err)
	}

	publisher := kafka.NewEventPublisher(kafkaProducer, schemaRegistry, slog.Default())

	orderID := uuid.MustParse("25065241-5e2e-4f3b-b55b-915496e48a1e")
	correlationID := uuid.MustParse("662d8af3-bc8e-48a1-b4d5-2dae6e1f2ec1")

	payloadMap := map[string]any{
		"order_id": orderID.String(),
		"reason":   domain.ReservationFailedReasonInsufficientStock.String(),
	}
	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		log.Fatalf("Ошибка маршалинга payload: %v", err)
	}

	mockEvent := &domain.OutboxEvent{
		ID:            uuid.New(),
		EventType:     "inventory.reservation-failed",
		AggregateType: "order",
		AggregateID:   orderID,
		CorrelationID: correlationID,
		Payload:       payloadBytes,
		CreatedAt:     time.Now(),
	}

	err = publisher.Publish(ctx, mockEvent)
	if err != nil {
		log.Fatalf("Ошибка отправки мока: %v", err)
	}

	log.Println("🎉 Имитационное сообщение успешно отправлено в топик inventory.reserved!")
}
