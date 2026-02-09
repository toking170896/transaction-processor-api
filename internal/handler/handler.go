package handler

import (
	"errors"
	"net/http"
	"transaction-processor/internal/model"
	"transaction-processor/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handler struct {
	transactionService service.TransactionService
	logger             zerolog.Logger
}

func NewHandler(txService service.TransactionService, logger zerolog.Logger) *Handler {
	return &Handler{
		transactionService: txService,
		logger:             logger,
	}
}

func (h *Handler) SetupRoutes() *gin.Engine {
	router := gin.New()

	// Middlewares
	router.Use(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		gin.Recovery(),
	)

	// Swagger and health checks
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API routes
	v1 := router.Group("/api/v1")

	transactions := v1.Group("/transactions")
	transactions.POST("", h.ProcessTransaction)
	transactions.GET("/user/:id", h.GetTransactionsByUser)

	users := v1.Group("/users")
	users.GET("/:id/balance", h.GetBalance)

	return router
}

func (h *Handler) handleError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_SERVER_ERROR"

	resp := model.ErrorResponse{Error: err.Error()}

	switch {
	case errors.Is(err, model.ErrInsufficientBalance):
		status = http.StatusBadRequest
		code = "INSUFFICIENT_BALANCE"
	case errors.Is(err, model.ErrInvalidAmount):
		status = http.StatusBadRequest
		code = "INVALID_AMOUNT"
	case errors.Is(err, model.ErrInvalidState):
		status = http.StatusBadRequest
		code = "INVALID_STATE"
	case errors.Is(err, model.ErrInvalidSourceType):
		status = http.StatusBadRequest
		code = "INVALID_SOURCE_TYPE"
	case errors.Is(err, model.ErrUserNotFound):
		status = http.StatusNotFound
		code = "USER_NOT_FOUND"
	case errors.Is(err, model.ErrTransactionNotFound):
		status = http.StatusNotFound
		code = "TRANSACTION_NOT_FOUND"
	case errors.Is(err, model.ErrDuplicateTransaction):
		status = http.StatusConflict
		code = "DUPLICATE_TRANSACTION"
		resp.Details = "Transaction ID already exists for a different user"
	}
	resp.Code = code

	if status == http.StatusInternalServerError {
		h.logger.Error().Err(err).Msg("internal server error")
	}

	c.JSON(status, resp)
}
