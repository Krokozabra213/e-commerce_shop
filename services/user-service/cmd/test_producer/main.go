package main

import (
	"context"
	"fmt"
	"log"
	"time"

	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
	)
	if err != nil {
		log.Fatalf("failed to create Kafka client: %v", err)
	}
	defer client.Close()

	schemaRegistryURL := "http://localhost:8081"
	srClient, err := infrakafka.NewSchemaRegistryClient(schemaRegistryURL)
	if err != nil {
		log.Fatalf("failed to connect to schema registry: %v", err)
	}
	defer srClient.Close()

	userID := uuid.New().String()
	email := fmt.Sprintf("test-user-%s@example.com", userID[:8])

	event := &eventsv1.UserCreated{
		User: &eventsv1.User{
			Id:        userID,
			Email:     email,
			Roles:     []string{"ROLE_USER", "ROLE_MANAGER"},
			CreatedAt: timestamppb.Now(),
		},
		EventTime: timestamppb.Now(),
	}

	topic := "user.created"

	schema, err := srClient.RegisterOrGetSchema(topic, eventsv1.EventsProtoSchema)
	if err != nil {
		log.Fatalf("failed to register schema in registry: %v", err)
	}

	value, err := srClient.Serialize(ctx, schema.ID(), event)
	if err != nil {
		log.Fatalf("failed to serialize message with schema registry: %v", err)
	}

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(userID),
		Value: value,
		Headers: []kgo.RecordHeader{
			{Key: "content-type", Value: []byte("application/x-protobuf")},
		},
	}

	results := client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		log.Fatalf("failed to produce message: %v", err)
	}

	fmt.Printf("\n✅ Successfully sent message to Kafka with Schema Registry!\n")
	fmt.Printf("  - Topic: %s\n", topic)
	fmt.Printf("  - Schema ID: %d\n", schema.ID())
	fmt.Printf("  - User ID (Key): %s\n", userID)
	fmt.Printf("  - Email: %s\n", email)
}
