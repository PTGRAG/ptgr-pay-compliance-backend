package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/segmentio/kafka-go"
)

var writer *kafka.Writer

type ComplianceEvent struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

func InitProducer() {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "kafka:9092" // Matches your docker-compose
	}

	writer = &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Topic:    "compliance-events",
		Balancer: &kafka.LeastBytes{},
	}
	log.Println("Kafka Producer Initialized on", broker)
}

func PublishComplianceEvent(transactionId, status string) error {
	if writer == nil {
		return fmt.Errorf("kafka writer is not initialized")
	}

	event := ComplianceEvent{
		TransactionID: transactionId,
		Status:        status,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(transactionId), // Ensures all events for a transaction go to same partition
		Value: eventBytes,
	}

	err = writer.WriteMessages(context.Background(), msg)
	if err != nil {
		log.Printf("Failed to publish kafka message: %v", err)
		return err
	}

	log.Printf("Published compliance event for tx %s: %s", transactionId, status)
	return nil
}
