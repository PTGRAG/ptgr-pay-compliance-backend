package main

import (
	"log"
	"net"
	"os"

	grpc_server "github.com/PTGRAG/compliance-backend/internal/grpc"
	mykafka "github.com/PTGRAG/compliance-backend/internal/kafka"
	pb "github.com/PTGRAG/compliance-backend/pkg/pb"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// The data structure we expect from Didit Webhooks
type DiditWebhookPayload struct {
	Type          string `json:"type"`            // e.g. "transaction.updated"
	TransactionId string `json:"transaction_id"` // Matches our ID
	Status        string `json:"status"`         // e.g. "APPROVED", "DECLINED"
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading env file, using default environments")
	}

	// 1. Initialize Kafka Producer
	mykafka.InitProducer()

	// 2. Start gRPC Server in a goroutine so it runs in the background
	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("failed to listen on 50051: %v", err)
		}

		grpcServer := grpc.NewServer()
		complianceService := grpc_server.NewComplianceServiceServer()
		pb.RegisterComplianceServiceServer(grpcServer, complianceService)

		// Register reflection service ONLY in development/staging
		// In production, we don't want to expose our API surface
		if os.Getenv("APP_ENV") != "production" {
			reflection.Register(grpcServer)
		}

		log.Println("gRPC Server listening on port 50051...")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// 3. Start Fiber HTTP Server on the main thread
	app := fiber.New()

	app.Get("/health", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// 4. The Webhook Endpoint that Didit calls!
	app.Post("/webhook/transaction", func(c fiber.Ctx) error {
		var payload DiditWebhookPayload
		if err := c.Bind().JSON(&payload); err != nil {
			log.Printf("Invalid webhook payload: %v", err)
			return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
		}

		log.Printf("Received Didit Webhook: %s for %s", payload.Status, payload.TransactionId)

		// Broadcast to Node.js via Kafka!
		err := mykafka.PublishComplianceEvent(payload.TransactionId, payload.Status)
		if err != nil {
			log.Printf("Kafka Error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "failed to broadcast event"})
		}

		return c.SendStatus(200) // Always return 200 OK so Didit knows we got it
	})

	log.Println("Fiber Server listening on port 3000...")
	log.Fatal(app.Listen(":3000"))
}
