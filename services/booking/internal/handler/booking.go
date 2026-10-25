package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"bookly/services/booking/internal/middleware"
	"bookly/services/booking/internal/service"
)

type BookingHandler struct {
	bookings *service.BookingService
}

func NewBookingHandler(bookings *service.BookingService) *BookingHandler {
	return &BookingHandler{bookings: bookings}
}

type createBookingRequest struct {
	SpaceID   int64     `json:"space_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON (times must be in RFC 3339 format)")
		return
	}

	booking, err := h.bookings.Create(
		r.Context(),
		middleware.UserID(r.Context()),
		req.SpaceID,
		req.StartTime,
		req.EndTime,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, booking)
}

func (h *BookingHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	bookings, err := h.bookings.ListMine(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, bookings)
}

func (h *BookingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	booking, err := h.bookings.Get(
		r.Context(),
		middleware.UserID(r.Context()),
		middleware.Role(r.Context()),
		id,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, booking)
}

func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	booking, err := h.bookings.Cancel(
		r.Context(),
		middleware.UserID(r.Context()),
		middleware.Role(r.Context()),
		id,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, booking)
}
