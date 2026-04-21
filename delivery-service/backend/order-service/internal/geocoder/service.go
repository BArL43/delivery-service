package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Service struct {
	cache    GeocodeCache
	primary  GeocodeProvider
	fallback GeocodeProvider
}

func NewService(cache GeocodeCache, primary GeocodeProvider, fallback GeocodeProvider) *Service {
	return &Service{
		cache:    cache,
		primary:  primary,
		fallback: fallback,
	}
}

func (s *Service) GetCoordinates(ctx context.Context, address string) (*Coordinates, string, error) {
	normAddr := NormalizeAddress(address)
	cacheKey := GenerateCacheKey(normAddr)

	coords, err := s.cache.Get(ctx, cacheKey)
	if err == nil && coords != nil {
		return coords, "redis_cache", nil
	}

	coords, err = s.primary.Geocode(ctx, normAddr)
	providerName := s.primary.Name()
	if err != nil {
		log.Printf("[Geocode] Внимание: провайдер %s упал (%v), переключаемся на %s", s.primary.Name(), err, s.fallback.Name())
		coords, err = s.fallback.Geocode(ctx, normAddr)
		providerName = s.fallback.Name()
		if err != nil {
			return nil, "", fmt.Errorf("both geocoders failed: primary error: %w, fallback error: %w", err, err)
		}
	}

	go func(c *Coordinates, key string) {
		_ = s.cache.Set(context.Background(), key, c)
	}(coords, cacheKey)

	return coords, providerName, nil
}

func (s *Service) SuggestAddress(ctx context.Context, query string, limit int) ([]AddressSuggestion, error) {
	suggest, err := s.primary.Suggest(ctx, query, limit)
	
	if err != nil {
		log.Printf("[Geocode] Внимание: провайдер %s упал при поиске подсказок (%v), переключаемся на %s", s.primary.Name(), err, s.fallback.Name())
		suggest, err = s.fallback.Suggest(ctx, query, limit)
		if err != nil {
			return nil, fmt.Errorf("both geocoders failed to suggest addresses: primary error: %w, fallback error: %w", err, err)
		}
	}
	
	return suggest, nil
}

func (s *Service) ReverseAddress(ctx context.Context, lat, lon float64) (*ReversResponse, error) {
	res, err := s.primary.ReverseGeocode(ctx, lat, lon)

	if err != nil {
		log.Printf("[Geocode] Внимание: провайдер %s упал при обратном геокодировании (%v), переключаемся на %s", s.primary.Name(), err, s.fallback.Name())
		res, err = s.fallback.ReverseGeocode(ctx, lat, lon)
		if err != nil {
			return nil, fmt.Errorf("both geocoders failed to reverse geocode: primary error: %w, fallback error: %w", err, err)
		}
	}
	return res, nil
}

type ReverseGeocodeRequest struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func (h *GeocodeHandler) ReverseGeocode(w http.ResponseWriter, r *http.Request) {
	var req ReverseGeocodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	if req.Lat < -90 || req.Lat > 90 || req.Lon < -180 || req.Lon > 180 {
		http.Error(w, `{"error": "invalid latitude or longitude values"}`, http.StatusBadRequest)
		return	
	}

	addressInfo, err := h.service.ReverseAddress(r.Context(), req.Lat, req.Lon)
	if err != nil {
		http.Error(w, `{"error": "failed to reverse geocode coordinates"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(addressInfo)
}
