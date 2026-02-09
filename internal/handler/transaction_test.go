package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"transaction-processor/internal/model"
	"transaction-processor/mocks/service"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandler_ProcessTransaction_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := mocks.NewTransactionService(t)
	logger := zerolog.Nop()
	h := NewHandler(mockSvc, logger)

	router := gin.New()
	router.POST("/transactions", h.ProcessTransaction)

	reqBody := model.TransactionRequest{
		TransactionID: "550e8400-e29b-41d4-a716-446655440000",
		Amount:        "100.00",
		State:         "win",
	}
	body, _ := json.Marshal(reqBody)

	mockSvc.On("ProcessTransaction", mock.Anything, mock.Anything, model.SourceType("game"), int64(1)).Return(&model.TransactionResponse{
		Status:  "success",
		Balance: "200.00",
		Message: "Success",
	}, nil)

	req, _ := http.NewRequest(http.MethodPost, "/transactions?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Source-Type", "game")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp model.TransactionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "200.00", resp.Balance)
}

func TestHandler_ProcessTransaction_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := mocks.NewTransactionService(t)
	h := NewHandler(mockSvc, zerolog.Nop())

	router := gin.New()
	router.POST("/transactions", h.ProcessTransaction)

	reqBody := model.TransactionRequest{
		TransactionID: "not-a-uuid",
		Amount:        "100.00",
		State:         "win",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/transactions?user_id=1", bytes.NewBuffer(body))
	req.Header.Set("Source-Type", "game")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp model.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_REQUEST", resp.Code)
}
