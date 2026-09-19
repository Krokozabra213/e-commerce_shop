package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type DLQProducer struct {
	client *kgo.Client
	topic  string
}

func NewDLQProducer(brokers []string, topic string) (*DLQProducer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(topic),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return nil, fmt.Errorf("create dlq producer: %w", err)
	}

	return &DLQProducer{
		client: client,
		topic:  topic,
	}, nil
}

func (d *DLQProducer) Send(ctx context.Context, original *kgo.Record, reason string) error {
	rec := &kgo.Record{
		Value: original.Value,
		Key:   original.Key,
		Headers: append(original.Headers,
			kgo.RecordHeader{Key: "dlq.original.topic", Value: []byte(original.Topic)},
			kgo.RecordHeader{Key: "dlq.original.partition", Value: []byte(fmt.Sprintf("%d", original.Partition))},
			kgo.RecordHeader{Key: "dlq.original.offset", Value: []byte(fmt.Sprintf("%d", original.Offset))},
			kgo.RecordHeader{Key: "dlq.error.reason", Value: []byte(reason)},
			kgo.RecordHeader{Key: "dlq.timestamp", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		),
	}

	return d.client.ProduceSync(ctx, rec).FirstErr()
}

func (d *DLQProducer) Close() {
	d.client.Close()
}

func (d *DLQProducer) Ping(ctx context.Context) error {
	return d.client.Ping(ctx)
}
