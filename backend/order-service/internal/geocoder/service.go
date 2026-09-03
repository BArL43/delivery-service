package geocoder

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Service struct {
	cache    GeocodeCache
	primary  GeocodeProvider
	fallback GeocodeProvider
}

func NewService(cache GeocodeCache, primary, fallback GeocodeProvider) *Service {
	return &Service{cache: cache, primary: primary, fallback: fallback}
}

func (s *Service) GetCoordinates(ctx context.Context, address string) (*Coordinates, string, error) {
	if s.primary == nil {
		return nil, "", errors.New("primary geocoder is not configured")
	}
	normalized := NormalizeAddress(address)
	cacheKey := GenerateCacheKey(normalized)

	if s.cache != nil {
		if coords, err := s.cache.Get(ctx, cacheKey); err == nil && coords != nil {
			return coords, "redis_cache", nil
		}
	}

	coords, primaryErr := s.primary.Geocode(ctx, normalized)
	provider := s.primary.Name()
	if primaryErr != nil {
		if s.fallback == nil {
			return nil, "", fmt.Errorf("geocoder %s failed: %w", s.primary.Name(), primaryErr)
		}
		coords, fallbackErr := s.fallback.Geocode(ctx, normalized)
		if fallbackErr != nil {
			return nil, "", fmt.Errorf("primary geocoder failed: %v; fallback geocoder failed: %w", primaryErr, fallbackErr)
		}
		provider = s.fallback.Name()
	}

	if s.cache != nil {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = s.cache.Set(cacheCtx, cacheKey, coords)
	}
	return coords, provider, nil
}

func (s *Service) SuggestAddress(ctx context.Context, query string, limit int) ([]AddressSuggestion, error) {
	if s.primary == nil {
		return nil, errors.New("primary geocoder is not configured")
	}
	result, primaryErr := s.primary.Suggest(ctx, query, limit)
	if primaryErr == nil {
		return result, nil
	}
	if s.fallback == nil {
		return nil, fmt.Errorf("geocoder %s failed: %w", s.primary.Name(), primaryErr)
	}
	result, fallbackErr := s.fallback.Suggest(ctx, query, limit)
	if fallbackErr != nil {
		return nil, fmt.Errorf("primary geocoder failed: %v; fallback geocoder failed: %w", primaryErr, fallbackErr)
	}
	return result, nil
}

func (s *Service) ReverseAddress(ctx context.Context, lat, lon float64) (*ReversResponse, error) {
	if s.primary == nil {
		return nil, errors.New("primary geocoder is not configured")
	}
	result, primaryErr := s.primary.ReverseGeocode(ctx, lat, lon)
	if primaryErr == nil {
		return result, nil
	}
	if s.fallback == nil {
		return nil, fmt.Errorf("geocoder %s failed: %w", s.primary.Name(), primaryErr)
	}
	result, fallbackErr := s.fallback.ReverseGeocode(ctx, lat, lon)
	if fallbackErr != nil {
		return nil, fmt.Errorf("primary geocoder failed: %v; fallback geocoder failed: %w", primaryErr, fallbackErr)
	}
	return result, nil
}
