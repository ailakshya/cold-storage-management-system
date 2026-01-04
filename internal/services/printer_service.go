package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	PrinterURL = "http://192.168.15.101:5000"
)

type PrinterService struct {
	client  *http.Client
	baseURL string
}

type PrintFullRequest struct {
	Line1  string `json:"line1"`
	Line2  string `json:"line2"`
	Font1  string `json:"font1"`
	Font2  string `json:"font2"`
	Copies int    `json:"copies"`
}

type PrintResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewPrinterService() *PrinterService {
	return &PrinterService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: PrinterURL,
	}
}

// Print2Up prints 2-up labels (side by side) with thock number and customer name
// copies is the exact number of labels wanted
func (s *PrinterService) Print2Up(thockNumber, customerName string, copies int) error {
	if copies < 1 {
		copies = 1
	}

	// 2-up prints 2 labels per sheet
	// For odd numbers, print (n-1)/2 sheets of 2-up, then 1 single label
	twoUpCopies := copies / 2
	singleLabel := copies % 2

	// Print 2-up labels if needed
	if twoUpCopies > 0 {
		req := PrintFullRequest{
			Line1:  thockNumber,
			Line2:  customerName,
			Font1:  "5",
			Font2:  "4",
			Copies: twoUpCopies,
		}
		if err := s.sendPrintRequest("/print-2up", req); err != nil {
			return err
		}
	}

	// Print single label if odd count
	if singleLabel > 0 {
		req := PrintFullRequest{
			Line1:  thockNumber,
			Line2:  customerName,
			Font1:  "5",
			Font2:  "4",
			Copies: 1,
		}
		if err := s.sendPrintRequest("/print-full", req); err != nil {
			return err
		}
	}

	return nil
}

// PrintReceipt sends HTML content to HP printer
func (s *PrinterService) PrintReceipt(html string) error {
	reqData := map[string]string{"html": html}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("failed to marshal receipt request: %w", err)
	}

	resp, err := s.client.Post(
		s.baseURL+"/print-receipt",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to send receipt request: %w", err)
	}
	defer resp.Body.Close()

	var printResp PrintResponse
	if err := json.NewDecoder(resp.Body).Decode(&printResp); err != nil {
		return fmt.Errorf("failed to decode receipt response: %w", err)
	}

	if !printResp.Success {
		return fmt.Errorf("receipt print failed: %s", printResp.Message)
	}

	return nil
}

func (s *PrinterService) sendPrintRequest(endpoint string, req PrintFullRequest) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal print request: %w", err)
	}

	resp, err := s.client.Post(
		s.baseURL+endpoint,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to send print request: %w", err)
	}
	defer resp.Body.Close()

	var printResp PrintResponse
	if err := json.NewDecoder(resp.Body).Decode(&printResp); err != nil {
		return fmt.Errorf("failed to decode print response: %w", err)
	}

	if !printResp.Success {
		return fmt.Errorf("print failed: %s", printResp.Message)
	}

	return nil
}

