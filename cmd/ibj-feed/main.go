// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/connector/mock"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

func main() {
	tenant := flag.String("tenant", "demo_store", "Tenant identifier")
	secret := flag.String("secret", "test_secret_123", "Tenant HMAC secret")
	endpoint := flag.String("endpoint", "http://localhost:8080/catalog/batches", "Target API endpoint")
	file := flag.String("file", "", "Path to batch JSON file")
	generate := flag.Bool("generate", false, "Generate synthetic batch instead of reading file")
	count := flag.Int("count", 100, "Number of variants to generate when --generate is set")
	dryRun := flag.Bool("dry-run", false, "Print signed headers and payload without sending")

	flag.Parse()

	if *file == "" && !*generate {
		fmt.Fprintf(os.Stderr, "Error: either --file or --generate must be specified\n")
		flag.Usage()
		os.Exit(1)
	}

	var payload []byte
	var err error

	if *file != "" {
		payload, err = os.ReadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", *file, err)
			os.Exit(1)
		}
	} else {
		batchID := fmt.Sprintf("batch_cli_%d", time.Now().Unix())
		payload, err = mock.GenerateBatch(*tenant, *count, batchID, "v1.0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating batch: %v\n", err)
			os.Exit(1)
		}
	}

	now := time.Now().Unix()
	sig := ingest.SignPayload(*secret, now, payload)

	if *dryRun {
		fmt.Printf("--- Dry Run ---\n")
		fmt.Printf("Endpoint: %s\n", *endpoint)
		fmt.Printf("X-IBJ-Tenant: %s\n", *tenant)
		fmt.Printf("X-IBJ-Timestamp: %d\n", now)
		fmt.Printf("X-IBJ-Signature: %s\n", sig)
		fmt.Printf("Payload size: %d bytes\n", len(payload))
		fmt.Printf("Payload:\n%s\n", string(payload))
		return
	}

	dispatcher := mock.NewDispatcher(*endpoint, *tenant, *secret, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	receipt, err := dispatcher.DispatchBatch(ctx, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dispatching batch: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Dispatched batch. Status: %d\nResponse: %s\n", receipt.StatusCode, string(receipt.RawBody))
	if receipt.StatusCode != 202 {
		os.Exit(1)
	}
}
