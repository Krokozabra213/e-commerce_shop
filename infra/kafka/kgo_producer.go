package infrakafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/twmb/franz-go/pkg/kgo"
)

type KGOProducer struct {
	client *kgo.Client
	logger *slog.Logger
}

func NewKGOProducer(cfg infracfg.KafkaProducerConfig, logger *slog.Logger) (*KGOProducer, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordRetries(cfg.MaxRetries),
		kgo.RetryBackoffFn(func(int) time.Duration { return cfg.RetryBackoff }),
		kgo.DialTimeout(cfg.DialTimeout),
		kgo.ProduceRequestTimeout(cfg.WriteTimeout),
		kgo.ProducerBatchCompression(kgo.ZstdCompression()),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(pingCtx); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping kafka brokers: %w", err)
	}

	logger.Info("kafka producer connected",
		"brokers", cfg.Brokers,
		"client_id", cfg.ClientID,
	)

	return &KGOProducer{
		client: client,
		logger: logger,
	}, nil
}

func (p *KGOProducer) ProduceSync(
	ctx context.Context,
	topic string,
	key, value []byte,
	headers map[string]string,
) error {
	record := &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	}

	for k, v := range headers {
		record.Headers = append(record.Headers, kgo.RecordHeader{
			Key:   k,
			Value: []byte(v),
		})
	}

	results := p.client.ProduceSync(ctx, record)

	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("produce to %s: %w", topic, err)
	}

	r := results[0].Record
	p.logger.Debug("kafka message sent",
		"topic", r.Topic,
		"partition", r.Partition,
		"offset", r.Offset,
	)

	return nil
}

func (p *KGOProducer) Ping(ctx context.Context) error {
	if err := p.client.Ping(ctx); err != nil {
		return fmt.Errorf("kafka ping failed: %w", err)
	}
	return nil
}

func (p *KGOProducer) Close() {
	p.client.Flush(context.Background())
	p.client.Close()
	p.logger.Info("kafka producer closed")
}
