package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type OSMProvider struct {
	client *http.Client
}

func NewOSMProvider() *OSMProvider {
	return &OSMProvider{client: &http.Client{}}
}

func (p *OSMProvider) Name() string {
	return "OSM_Nominatim"
}

type osmResponse struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

func (p *OSMProvider) Geocode(ctx context.Context, address string) (*Coordinates, error) {
	baseURL := "https://nominatim.openstreetmap.org/search"
	requestURL := fmt.Sprintf("%s?q=%s&format=json&limit=1", baseURL, url.QueryEscape(address))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSM request: %w", err)
	}

	req.Header.Set("User-Agent", "DeliveryService/1.0 (test-student-project))")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSM request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSM returned non-200 status: %d", resp.StatusCode)
	}

	var result []osmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode OSM response: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("OSM: no results found for address: %s", address)
	}

	var coords Coordinates
	_, err = fmt.Sscanf(result[0].Lat, "%f", &coords.Latitude)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latitude: %w", err)
	}

	_, err = fmt.Sscanf(result[0].Lon, "%f", &coords.Longitude)
	if err != nil {
		return nil, fmt.Errorf("failed to parse longitude: %w", err)
	}

	return &coords, nil
}

type osmSuggestResponse struct {
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	DisplayName string  `json:"display_name"`
	Importance  float64 `json:"importance"`
}

func (p *OSMProvider) Suggest(ctx context.Context, query string, limit int) ([]AddressSuggestion, error) {
	baseURL := "https://nominatim.openstreetmap.org/search"
	requestURL := fmt.Sprintf("%s?q=%s&format=json&limit=%d", baseURL, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSM suggest request: %w", err)
	}

	req.Header.Set("User-Agent", "DeliveryService/1.0 (test-student-project)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSM suggest request error: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSM suggest returned non-200 status: %d", resp.StatusCode)
	}

	var osmResult []osmSuggestResponse
	if err := json.NewDecoder(resp.Body).Decode(&osmResult); err != nil {
		return nil, fmt.Errorf("failed to decode OSM suggest response: %w", err)
	}

	var suggestions []AddressSuggestion
	for _, res := range osmResult {
		var lat, lon float64
		if _, err = fmt.Sscanf(res.Lat, "%f", &lat); err != nil {
			continue
		}
		if _, err = fmt.Sscanf(res.Lon, "%f", &lon); err != nil {
			continue
		}
		suggestions = append(suggestions, AddressSuggestion{
			DisplayName: res.DisplayName,
			Lat:         lat,
			Lon:         lon,
			Confidence:  res.Importance,
		})
	}

	if suggestions == nil {
		suggestions = make([]AddressSuggestion, 0)
	}

	return suggestions, nil
}

type osmReverseResponse struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		HouseNumber  string `json:"house_number"`
		Road         string `json:"road"`
		City         string `json:"city"`
		Town         string `json:"town"`
		Vallage      string `json:"village"`
		CityDistrict string `json:"city_district"`
	} `json:"address"`
}

func (p *OSMProvider) ReverseGeocode(ctx context.Context, lat, lon float64) (*ReversResponse, error) {
	baseURL := "https://nominatim.openstreetmap.org/reverse"
	requestURL := fmt.Sprintf("%s?lat=%f&lon=%f&format=json", baseURL, lat, lon)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSM reverse geocode request: %w", err)
	}

	req.Header.Set("User-Agent", "DeliveryService/1.0 (test-student-project)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSM reverse geocode request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSM reverse geocode returned non-200 status: %d", resp.StatusCode)
	}

	var osmResult osmReverseResponse
	if err := json.NewDecoder(resp.Body).Decode(&osmResult); err != nil {
		return nil, fmt.Errorf("failed to decode OSM reverse geocode response: %w", err)
	}

	if osmResult.DisplayName == "" {
		return nil, fmt.Errorf("OSM reverse geocode: no address found for coordinates: %f, %f", lat, lon)
	}

	city := osmResult.Address.City
	if city == "" {
		city = osmResult.Address.Town
	}
	if city == "" {
		city = osmResult.Address.Vallage
	}

	return &ReversResponse{
		Address: osmResult.DisplayName,
		House:   osmResult.Address.HouseNumber,
		Street:  osmResult.Address.Road,
		City:    city,
	}, nil

}
