package handler

import (
	"encoding/json"
	"net/http"

	"bookly/services/booking/internal/middleware"
	"bookly/services/booking/internal/service"
)

type SpaceHandler struct {
	spaces *service.SpaceService
}

func NewSpaceHandler(spaces *service.SpaceService) *SpaceHandler {
	return &SpaceHandler{spaces: spaces}
}

type createSpaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Capacity    int    `json:"capacity"`
}

func (h *SpaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	space, err := h.spaces.Create(
		r.Context(),
		middleware.Role(r.Context()),
		req.Name,
		req.Description,
		req.Capacity,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, space)
}

func (h *SpaceHandler) List(w http.ResponseWriter, r *http.Request) {
	spaces, err := h.spaces.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, spaces)
}

func (h *SpaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid space id")
		return
	}

	space, err := h.spaces.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, space)
}
