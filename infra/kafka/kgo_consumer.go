package infrakafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/twmb/franz-go/pkg/kgo"
)

type RecordHandler func(ctx context.Context, record *kgo.Record) error

type DLQ interface {
	Send(ctx context.Context, original *kgo.Record, reason string) error
}

type KGOConsumer struct {
	client  *kgo.Client
	dlq     DLQ
	logger  *slog.Logger
	handler RecordHandler
	topic   string
}

func NewKGOConsumer(
	cfg infracfg.KafkaConsumerConfig,
	handler RecordHandler,
	logger *slog.Logger,
	dlq DLQ,
) (*KGOConsumer, error) {
	offset := kgo.NewOffset().AtStart()
	if !cfg.StartOffsetEarliest {
		offset = kgo.NewOffset().AtEnd()
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topic),
		kgo.ConsumeResetOffset(offset),

		kgo.SessionTimeout(cfg.SessionTimeout),
		kgo.HeartbeatInterval(cfg.HeartbeatInterval),
		kgo.RebalanceTimeout(cfg.MaxPollInterval),

		kgo.BlockRebalanceOnPoll(),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping kafka brokers: %w", err)
	}

	logger.Info("kafka consumer connected",
		"brokers", cfg.Brokers,
		"group_id", cfg.GroupID,
		"topic", cfg.Topic,
		"start_offset_earliest", cfg.StartOffsetEarliest,
	)

	return &KGOConsumer{
		client:  client,
		dlq:     dlq,
		logger:  logger,
		handler: handler,
		topic:   cfg.Topic,
	}, nil
}

func (c *KGOConsumer) Run(ctx context.Context) error {
	c.logger.Info("kafka consumer started", "topic", c.topic)
	defer c.logger.Info("kafka consumer stopped")

	for {
		if ctx.Err() != nil {
			return nil
		}

		fetches := c.client.PollFetches(ctx)

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				if errors.Is(e.Err, context.Canceled) {
					return nil
				}
				c.logger.Error("kafka fetch error",
					"topic", e.Topic,
					"partition", e.Partition,
					"error", e.Err,
				)
			}
			continue
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()

			if err := c.handler(ctx, record); err != nil {
				c.logger.Error("failed to process record",
					"topic", record.Topic,
					"partition", record.Partition,
					"offset", record.Offset,
					"key", string(record.Key),
					"error", err,
				)

				if dlqErr := c.dlq.Send(ctx, record, err.Error()); dlqErr != nil {
					c.logger.Error("failed to send to DLQ, skipping record",
						"topic", record.Topic,
						"partition", record.Partition,
						"offset", record.Offset,
						"dlq_error", dlqErr,
					)
				}

				c.client.MarkCommitRecords(record)
				continue
			}

			c.client.MarkCommitRecords(record)
		}

		if err := c.client.CommitMarkedOffsets(ctx); err != nil {
			c.logger.Error("failed to commit offsets", "error", err)
		}

		c.client.AllowRebalance()
	}
}

func (c *KGOConsumer) Close() {
	c.client.Close()
	c.logger.Info("kafka consumer closed")
}

func (c *KGOConsumer) Ping(ctx context.Context) error {
	return c.client.Ping(ctx)
}
