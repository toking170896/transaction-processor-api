package handler

import (
	"net/http"
	"strconv"
	"transaction-processor/internal/model"

	"github.com/gin-gonic/gin"
)

// ProcessTransaction
// @Summary Process a transaction
// @Description Process a win/lost transaction from third-party provider
// @Tags transactions
// @Accept json
// @Produce json
// @Param Source-Type header string true "Source type" Enums(game, server, payment)
// @Param user_id query int true "User ID"
// @Param transaction body model.TransactionRequest true "Transaction details"
// @Success 200 {object} model.TransactionResponse "Already processed"
// @Success 201 {object} model.TransactionResponse "Created"
// @Failure 400 {object} model.ErrorResponse "Bad request"
// @Failure 409 {object} model.ErrorResponse "Conflict"
// @Router /transactions [post]
func (h *Handler) ProcessTransaction(c *gin.Context) {
	sourceTypeHeader := c.GetHeader("Source-Type")
	sourceType, err := model.ParseSourceType(sourceTypeHeader)
	if err != nil {
		h.handleError(c, err)
		return
	}

	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "user_id query parameter is required",
			Code:  "INVALID_REQUEST",
		})
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "user_id must be a positive integer",
			Code:  "INVALID_REQUEST",
		})
		return
	}

	var req model.TransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "Invalid request body",
			Code:  "INVALID_REQUEST",
		})
		return
	}

	resp, err := h.transactionService.ProcessTransaction(c.Request.Context(), &req, sourceType, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	statusCode := http.StatusCreated
	if resp.Status == "already_processed" {
		statusCode = http.StatusOK
	}
	c.JSON(statusCode, resp)
}

// GetBalance
// @Summary Get user balance
// @Description Returns the current balance for a user
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} model.BalanceResponse
// @Failure 404 {object} model.ErrorResponse "User not found"
// @Router /users/{id}/balance [get]
func (h *Handler) GetBalance(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.handleError(c, model.ErrUserNotFound)
		return
	}

	resp, err := h.transactionService.GetBalance(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTransactionsByUser
// @Summary Get user transactions
// @Description Returns a paginated list of transactions for a user
// @Tags transactions
// @Produce json
// @Param id path int true "User ID"
// @Param limit query int false "Limit" default(10)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} model.TransactionListResponse
// @Failure 404 {object} model.ErrorResponse "User not found"
// @Router /transactions/user/{id} [get]
func (h *Handler) GetTransactionsByUser(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.handleError(c, model.ErrUserNotFound)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	transactions, err := h.transactionService.GetTransactionsByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.TransactionListResponse{
		Transactions: transactions,
		Total:        len(transactions),
		Limit:        limit,
		Offset:       offset,
	})
}
