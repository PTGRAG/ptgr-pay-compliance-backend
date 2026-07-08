package grpc_server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	pb "github.com/PTGRAG/compliance-backend/pkg/pb"
)

type ComplianceServer struct {
	pb.UnimplementedComplianceServiceServer
}

func (s *ComplianceServer) ScreenTransaction(ctx context.Context, req *pb.ScreenTransactionRequest) (*pb.ScreenTransactionResponse, error) {
	log.Printf("Screening transaction %s for %s", req.TransactionId, req.UserFullName)

	// 1. Build the base Didit payload
	payload := map[string]interface{}{
		"transaction_id":       req.TransactionId,
		"transaction_category": "finance",
		"transaction_details": map[string]interface{}{
			"direction":     "outbound",
			"amount":        req.Amount,
			"currency":      req.Currency,
			"currency_kind": "crypto", 
			"action_type":   "withdrawal",
		},
		"subject": map[string]interface{}{
			"entity_type": "individual",
			"vendor_data": req.UserId,
			"full_name":   req.UserFullName,
			"payment_method": map[string]interface{}{
				"method_type": "crypto_wallet",
				"account_id":  req.SenderWallet, 
			},
		},
	}

	// 2. Dynamically add Travel Rule details if Node.js requested it!
	if req.RequiresTravelRule {
		payload["transaction_category"] = "travel_rule" // Didit expects this category
		payload["travel_rule_details"] = map[string]interface{}{
			"status":   "PENDING",
			"protocol": "IVMS101",
			"required": true,
			"originator_data": map[string]interface{}{
				"name": req.UserFullName,
			},
			"beneficiary_data": map[string]interface{}{
				"name": req.BeneficiaryName,
			},
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal didit payload: %v", err)
	}

	// 3. Make the HTTP POST Request
	diditReq, err := http.NewRequest("POST", "https://verification.didit.me/v3/transactions/", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %v", err)
	}

	diditReq.Header.Set("x-api-key", os.Getenv("DIDIT_API_KEY"))
	diditReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(diditReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call didit api: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		log.Printf("Didit API Error: %s", string(body))
		return &pb.ScreenTransactionResponse{
			IsApproved:   false,
			RiskLevel:    "CRITICAL",
			ErrorMessage: "Didit API rejected the transaction request",
		}, nil
	}

	var diditRes map[string]interface{}
	json.Unmarshal(body, &diditRes)

	status := diditRes["status"].(string)
	severity := "UNKNOWN"
	if diditRes["severity"] != nil {
		severity = diditRes["severity"].(string)
	}

	log.Printf("Didit Success! Status=%s, Severity=%s", status, severity)

	return &pb.ScreenTransactionResponse{
		IsApproved:    status == "APPROVED",
		RiskLevel:     severity,
		DiditRecordId: diditRes["uuid"].(string),
	}, nil
}

func NewComplianceServiceServer() *ComplianceServer {
	return &ComplianceServer{}
}
