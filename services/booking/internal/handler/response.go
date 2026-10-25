package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"bookly/services/booking/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeServiceError maps errors from the service layer to HTTP responses.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, service.ErrSpaceNotFound):
		writeError(w, http.StatusNotFound, "space not found")
	case errors.Is(err, service.ErrBookingNotFound):
		writeError(w, http.StatusNotFound, "booking not found")
	case errors.Is(err, service.ErrSpaceExists),
		errors.Is(err, service.ErrSlotTaken),
		errors.Is(err, service.ErrAlreadyCancelled):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
