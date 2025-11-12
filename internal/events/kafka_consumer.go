package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader *kafka.Reader
	hub    *Hub
	log    *slog.Logger
}

type KafkaConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
	MaxWait time.Duration
}

func NewKafkaConsumer(cfg KafkaConsumerConfig, hub *Hub, log *slog.Logger) (*KafkaConsumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers list is empty")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka topic is required")
	}
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("kafka group id is required")
	}
	maxWait := cfg.MaxWait
	if maxWait <= 0 {
		maxWait = 500 * time.Millisecond
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		StartOffset: kafka.LastOffset,
		MaxWait:     maxWait,
	})
	return &KafkaConsumer{
		reader: reader,
		hub:    hub,
		log:    log,
	}, nil
}

func (c *KafkaConsumer) Run(ctx context.Context) {
	go func() {
		for {
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				c.log.Warn("kafka read failed", slog.String("err", err.Error()))
				time.Sleep(500 * time.Millisecond)
				continue
			}
			jobID, ok := extractJobID(msg.Value)
			if !ok {
				continue
			}
			c.hub.Publish(jobID, msg.Value)
		}
	}()
}

func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}

type jobEnvelope struct {
	Job struct {
		ID string `json:"id"`
	} `json:"job"`
}

func extractJobID(payload []byte) (string, bool) {
	var env jobEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return "", false
	}
	if env.Job.ID == "" {
		return "", false
	}
	return env.Job.ID, true
}
