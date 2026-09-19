package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
)

func main() {
	client, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

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

	value, err := proto.Marshal(event)
	if err != nil {
		log.Fatalf("failed to marshal proto: %v", err)
	}

	record := &kgo.Record{
		Topic: "user.created",
		Key:   []byte(userID),
		Value: value,
		Headers: []kgo.RecordHeader{
			{Key: "content-type", Value: []byte("application/protobuf")},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results := client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		log.Fatalf("failed to produce message: %v", err)
	}

	fmt.Printf("✅ Successfully sent message to Kafka!\n")
	fmt.Printf("  - Topic: user.created\n")
	fmt.Printf("  - User ID (Key): %s\n", userID)
	fmt.Printf("  - Email: %s\n", email)
}
