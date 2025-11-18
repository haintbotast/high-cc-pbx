package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
	}
}

// respondXML sends an XML response
func respondXML(w http.ResponseWriter, statusCode int, xml string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(statusCode)
	w.Write([]byte(xml))
}

// respondError sends an error response
func respondError(w http.ResponseWriter, statusCode int, message string, err error) {
	log.Printf("API Error [%d]: %s - %v", statusCode, message, err)

	response := &models.APIResponse{
		Success: false,
		Message: message,
		Error: &models.APIError{
			Code:    fmt.Sprintf("E%d", statusCode),
			Message: message,
		},
	}

	if err != nil {
		response.Error.Details = map[string]string{
			"error": err.Error(),
		}
	}

	respondJSON(w, statusCode, response)
}

// respondSuccess sends a success response
func respondSuccess(w http.ResponseWriter, message string, data interface{}) {
	response := &models.APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	}

	respondJSON(w, http.StatusOK, response)
}

// errValidation creates a validation error
func errValidation(message string) error {
	return fmt.Errorf("validation error: %s", message)
}
