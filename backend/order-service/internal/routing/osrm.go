package routing

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	baseURL string
	client  *http.Client
}

func NewHandler(baseURL string) *Handler {
	return &Handler{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

func (h *Handler) Route(w http.ResponseWriter, r *http.Request) {
	fromLat, err := coordinate(r, "fromLat", -90, 90)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	fromLon, err := coordinate(r, "fromLon", -180, 180)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	toLat, err := coordinate(r, "toLat", -90, 90)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	toLon, err := coordinate(r, "toLon", -180, 180)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	requestURL := fmt.Sprintf("%s/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson", h.baseURL, fromLon, fromLat, toLon, toLat)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, requestURL, nil)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to prepare routing request")
		return
	}
	resp, err := h.client.Do(req)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "routing provider unavailable")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		jsonError(w, http.StatusBadGateway, "routing provider returned an error")
		return
	}
	var payload struct {
		Routes []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
			Geometry any     `json:"geometry"`
		} `json:"routes"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil || len(payload.Routes) == 0 {
		jsonError(w, http.StatusBadGateway, "invalid routing response")
		return
	}
	writeJSON(w, http.StatusOK, payload.Routes[0])
}

func coordinate(r *http.Request, key string, minValue, maxValue float64) (float64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, fmt.Errorf("missing query parameter: %s", key)
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < minValue || value > maxValue {
		return 0, fmt.Errorf("invalid query parameter: %s", key)
	}
	return value, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
