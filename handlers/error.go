package handlers

import (
	"log"
	"net/http"

	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// CustomHTTPErrorHandler is the central error handler for the Echo application.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	// Attempt to cast the error to our custom AppError type.
	appErr, ok := err.(*utils.AppError)
	if !ok {
		// If it's a different type of error (e.g., from Echo itself), handle it generically.
		log.Printf("[Unhandled Error] Type: %T, Error: %v", err, err)
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("An unexpected internal error occurred."))
		return
	}

	// If there's an underlying original error, log it for debugging purposes.
	if internalErr := appErr.Unwrap(); internalErr != nil {
		log.Printf("[Error] Code: %d, Message: %s, InternalDetails: %v", appErr.Code, appErr.Message, internalErr)
	}

	// Respond to the client with the code and message defined in the AppError.
	c.JSON(appErr.Code, utils.ErrorResponse(appErr.Message))
}
