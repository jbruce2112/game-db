package api

import (
	"encoding/json"
	"net/http"
	"time"

	"game-db/internal/model"
)

func (h *Handler) sync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Cursor  int64        `json:"cursor"`
		Changes []model.Item `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Changes == nil {
		body.Changes = []model.Item{}
	}
	res, err := h.store.Sync(r.Context(), body.Cursor, body.Changes, time.Now())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
