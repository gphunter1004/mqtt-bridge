package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

var errorLogger *slog.Logger

// SetErrorLogger sets the logger for error handling.
func SetErrorLogger(logger *slog.Logger) {
	errorLogger = logger.With("component", "error_handler")
}

// CustomHTTPErrorHandler is the central error handler for the Echo application.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	// Create logger with request context if available
	var logger *slog.Logger

	// Attempt to cast the error to our custom AppError type.
	appErr, ok := err.(*utils.AppError)
	if !ok {
		// If it's a different type of error (e.g., from Echo itself), handle it generically.
		//log.Printf("[Unhandled Error] Type: %T, Error: %v", err, err)
		logger.Error("Unhandled error occurred",
			"error_type", fmt.Sprintf("%T", err),
			"error_message", err.Error(),
			slog.Any("error", err))

		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("An unexpected internal error occurred."))
		return
	}

	// If there's an underlying original error, log it for debugging purposes.
	if internalErr := appErr.Unwrap(); internalErr != nil {
		//log.Printf("[Error] Code: %d, Message: %s, InternalDetails: %v", appErr.Code, appErr.Message, internalErr)
		logger.Info("Error handled",
			"status_code", appErr.Code,
			"error_message", appErr.Message,
			slog.Any("internal_error", internalErr))
	}

	// Respond to the client with the code and message defined in the AppError.
	c.JSON(appErr.Code, utils.ErrorResponse(appErr.Message))
}
