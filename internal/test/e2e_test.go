package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"transaction-processor/internal/config"
	"transaction-processor/internal/database"
	"transaction-processor/internal/handler"
	"transaction-processor/internal/model"
	"transaction-processor/internal/repository/postgres"
	"transaction-processor/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPool *pgxpool.Pool

const testUserID = 4

// Runs as first function
func TestMain(m *testing.M) {
	if os.Getenv("SKIP_E2E") != "" {
		fmt.Println("Skipping E2E tests")
		os.Exit(0)
	}

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	testPool = pool
	os.Exit(m.Run())
}

func setupE2E(t *testing.T) *handler.Handler {
	if testPool == nil {
		t.Skip("Database connection not available")
	}

	ctx := context.Background()
	_, err := testPool.Exec(ctx, "DELETE FROM transactions WHERE user_id = $1", testUserID)
	require.NoError(t, err)

	// Seed test user, update balance and version if already exists
	_, err = testPool.Exec(ctx, `
		INSERT INTO users (id, balance, version)
		VALUES ($1, 100.00, 0)
		ON CONFLICT (id) DO UPDATE
		SET balance = EXCLUDED.balance,
			version = EXCLUDED.version,
			updated_at = NOW()
	`, testUserID)
	require.NoError(t, err)

	logger := zerolog.Nop()
	userRepo := postgres.NewUserRepository(testPool)
	transRepo := postgres.NewTransactionRepository(testPool)
	dbManager := postgres.NewTransactionManager(testPool)

	txService := service.NewTransactionService(userRepo, transRepo, dbManager, logger)

	return handler.NewHandler(txService, logger)
}

// Test_ConcurrentRequests_SameTransactionID_DuplicateAndBalanceCorrect verifies:
// - Duplicated concurrent requests with the same transaction_id
// - One transaction processes successfully
// - All other requests receive "already_processed" status
// - Final balance is correct (updated only once)
// - No 500 errors occur
// - All goroutines start simultaneously via barrier channel
func Test_ConcurrentRequests_SameTransactionID_DuplicateAndBalanceCorrect(t *testing.T) {
	h := setupE2E(t)
	router := h.SetupRoutes()

	const (
		numRequests          = 25
		winAmount            = "10.00"
		expectedFinalBalance = "110.00"
	)

	// Use the same transaction ID for all requests
	transID := uuid.New().String()

	reqBody, err := json.Marshal(model.TransactionRequest{
		State:         "win",
		Amount:        winAmount,
		TransactionID: transID,
	})
	require.NoError(t, err)

	// Channel to synchronize goroutine start
	barrier := make(chan struct{})

	// Channel to collect responses
	type result struct {
		statusCode int
		response   model.TransactionResponse
	}
	results := make(chan result, numRequests)

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()

			// Wait for barrier to open
			<-barrier

			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/transactions?user_id=%d", testUserID), bytes.NewBuffer(reqBody))
			req.Header.Set("Source-Type", "game")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var resp model.TransactionResponse
			json.Unmarshal(w.Body.Bytes(), &resp)

			results <- result{
				statusCode: w.Code,
				response:   resp,
			}
		}()
	}

	// All goroutines start simultaneously
	close(barrier)

	// Wait for all requests to complete
	wg.Wait()
	close(results)

	var (
		successCount          int
		alreadyProcessedCount int
		errorCount            int
	)

	for res := range results {
		// No 500 errors occur
		assert.NotEqual(t, http.StatusInternalServerError, res.statusCode, "No 500 errors")
		// No 409 conflicts occur for same user
		assert.NotEqual(t, http.StatusConflict, res.statusCode, "No 409 error for same user/transaction")

		switch {
		case res.statusCode == http.StatusCreated && res.response.Status == "success":
			successCount++
		case res.statusCode == http.StatusOK && res.response.Status == "already_processed":
			alreadyProcessedCount++
		default:
			errorCount++
			t.Logf("Unexpected response: status=%d, body=%+v", res.statusCode, res.response)
		}
	}

	assert.Equal(t, 1, successCount, "Exactly one request should succeed with status=success")
	assert.Equal(t, numRequests-1, alreadyProcessedCount, "All other requests should return status=already_processed")
	assert.Equal(t, 0, errorCount, "No unexpected errors should occur")

	var dbBalance string
	err = testPool.QueryRow(context.Background(), "SELECT balance FROM users WHERE id = $1", testUserID).Scan(&dbBalance)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, dbBalance, "Balance should be updated exactly once")
}

// Test_ConcurrentRequests_MixedTransactionIDs_PartialDuplicate verifies:
// - Concurrent processing with both unique and duplicate transaction IDs
// - 5 requests share the same transaction_id (expect 1 success + 4 idempotent/conflict responses)
// - 20 requests have unique transaction_ids (expect 20 successes)
// - Final balance reflects exactly 21 unique transactions: 100 + (21 * 10) = 310.00
// - All goroutines start simultaneously via barrier channel
func Test_ConcurrentRequests_MixedTransactionIDs_PartialDuplicate(t *testing.T) {
	h := setupE2E(t)
	router := h.SetupRoutes()

	const (
		numRequests          = 25
		numDuplicates        = 5
		winAmount            = "10.00"
		expectedFinalBalance = "310.00" // 100 + (21 * 10)
	)

	// 1 shared ID for 5 requests, 20 unique IDs
	sharedTransID := uuid.New().String()
	transactionIDs := make([]string, numRequests)
	for i := 0; i < numDuplicates; i++ {
		transactionIDs[i] = sharedTransID
	}
	for i := numDuplicates; i < numRequests; i++ {
		transactionIDs[i] = uuid.New().String()
	}

	// Channel to synchronize goroutine start
	barrier := make(chan struct{})

	// Channel to collect responses
	type result struct {
		statusCode int
		response   model.TransactionResponse
	}
	results := make(chan result, numRequests)

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		transID := transactionIDs[i]
		go func() {
			defer wg.Done()

			// Wait for barrier to open
			<-barrier

			reqBody, _ := json.Marshal(model.TransactionRequest{
				State:         "win",
				Amount:        winAmount,
				TransactionID: transID,
			})

			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/transactions?user_id=%d", testUserID), bytes.NewBuffer(reqBody))
			req.Header.Set("Source-Type", "game")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var resp model.TransactionResponse
			json.Unmarshal(w.Body.Bytes(), &resp)

			results <- result{
				statusCode: w.Code,
				response:   resp,
			}
		}()
	}

	// All goroutines start simultaneously
	close(barrier)

	// Wait for all requests to complete
	wg.Wait()
	close(results)

	var (
		successCount          int
		alreadyProcessedCount int
		conflictOrErrorCount  int
	)

	for res := range results {
		// No 500 errors should occur
		assert.NotEqual(t, http.StatusInternalServerError, res.statusCode, "Should not return 500")
		// No 409 conflicts should occur for same user
		assert.NotEqual(t, http.StatusConflict, res.statusCode, "Should not return 409 for same user/transaction")

		switch {
		case res.statusCode == http.StatusCreated && res.response.Status == "success":
			successCount++
		case res.statusCode == http.StatusOK && res.response.Status == "already_processed":
			alreadyProcessedCount++
		default:
			conflictOrErrorCount++
			t.Logf("Unexpected response: status=%d, body=%+v", res.statusCode, res.response)
		}
	}

	// 21 successful unique transactions
	// 4 duplicate requests should all return 200 already_processed
	assert.Equal(t, 21, successCount, "successful transactions)")
	assert.Equal(t, numDuplicates-1, alreadyProcessedCount,
		"4 already_processed responses from duplicate transaction_id")
	assert.Equal(t, 0, conflictOrErrorCount, "No unexpected errors occur")

	t.Logf("Results: %d success, %d already_processed, %d errors",
		successCount, alreadyProcessedCount, conflictOrErrorCount)

	var dbBalance string
	err := testPool.QueryRow(context.Background(), "SELECT balance FROM users WHERE id = $1", testUserID).Scan(&dbBalance)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, dbBalance, "Balance should reflect exactly 21 unique transactions")
}

// Test_BasicTransactionFlow verifies basic win/lost functionality
func Test_BasicTransactionFlow(t *testing.T) {
	h := setupE2E(t)
	router := h.SetupRoutes()

	t.Run("Win transaction increases balance", func(t *testing.T) {
		transID := uuid.New().String()
		reqBody, _ := json.Marshal(model.TransactionRequest{
			State:         "win",
			Amount:        "25.50",
			TransactionID: transID,
		})

		req, _ := http.NewRequest("POST", "/api/v1/transactions?user_id=1", bytes.NewBuffer(reqBody))
		req.Header.Set("Source-Type", "game")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp model.TransactionResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, "125.50", resp.Balance)
	})

	t.Run("Lost transaction decreases balance", func(t *testing.T) {
		transID := uuid.New().String()
		reqBody, _ := json.Marshal(model.TransactionRequest{
			State:         "lost",
			Amount:        "15.50",
			TransactionID: transID,
		})

		req, _ := http.NewRequest("POST", "/api/v1/transactions?user_id=1", bytes.NewBuffer(reqBody))
		req.Header.Set("Source-Type", "game")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp model.TransactionResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, "110.00", resp.Balance)
	})

	t.Run("Insufficient balance returns error", func(t *testing.T) {
		transID := uuid.New().String()
		reqBody, _ := json.Marshal(model.TransactionRequest{
			State:         "lost",
			Amount:        "200.00",
			TransactionID: transID,
		})

		req, _ := http.NewRequest("POST", "/api/v1/transactions?user_id=1", bytes.NewBuffer(reqBody))
		req.Header.Set("Source-Type", "game")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var errResp model.ErrorResponse
		json.Unmarshal(w.Body.Bytes(), &errResp)
		assert.Equal(t, "INSUFFICIENT_BALANCE", errResp.Code)
	})
}
