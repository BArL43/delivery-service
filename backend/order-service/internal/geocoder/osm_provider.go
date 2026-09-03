package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const nominatimBaseURL = "https://nominatim.openstreetmap.org"

type OSMProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

func NewOSMProvider() *OSMProvider {
	return &OSMProvider{
		client:    &http.Client{Timeout: 10 * time.Second},
		baseURL:   nominatimBaseURL,
		userAgent: "delivery-service/1.0 (educational backend project)",
	}
}

func (p *OSMProvider) Name() string { return "OSM_Nominatim" }

type osmSearchResponse struct {
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	DisplayName string  `json:"display_name"`
	Importance  float64 `json:"importance"`
}

func (p *OSMProvider) Geocode(ctx context.Context, address string) (*Coordinates, error) {
	items, err := p.search(ctx, address, 1)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("OSM returned no results")
	}
	lat, lon, err := parseLatLon(items[0].Lat, items[0].Lon)
	if err != nil {
		return nil, err
	}
	return &Coordinates{Latitude: lat, Longitude: lon}, nil
}

func (p *OSMProvider) Suggest(ctx context.Context, query string, limit int) ([]AddressSuggestion, error) {
	items, err := p.search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	result := make([]AddressSuggestion, 0, len(items))
	for _, item := range items {
		lat, lon, err := parseLatLon(item.Lat, item.Lon)
		if err != nil {
			continue
		}
		result = append(result, AddressSuggestion{DisplayName: item.DisplayName, Lat: lat, Lon: lon, Confidence: item.Importance})
	}
	return result, nil
}

func (p *OSMProvider) search(ctx context.Context, query string, limit int) ([]osmSearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "jsonv2")
	params.Set("limit", strconv.Itoa(limit))
	requestURL := p.baseURL + "/search?" + params.Encode()
	var result []osmSearchResponse
	if err := p.doJSON(ctx, requestURL, &result); err != nil {
		return nil, err
	}
	return result, nil
}

type osmReverseResponse struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		HouseNumber string `json:"house_number"`
		Road        string `json:"road"`
		City        string `json:"city"`
		Town        string `json:"town"`
		Village     string `json:"village"`
	} `json:"address"`
}

func (p *OSMProvider) ReverseGeocode(ctx context.Context, lat, lon float64) (*ReversResponse, error) {
	params := url.Values{}
	params.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	params.Set("lon", strconv.FormatFloat(lon, 'f', 6, 64))
	params.Set("format", "jsonv2")
	var result osmReverseResponse
	if err := p.doJSON(ctx, p.baseURL+"/reverse?"+params.Encode(), &result); err != nil {
		return nil, err
	}
	if result.DisplayName == "" {
		return nil, fmt.Errorf("OSM returned no reverse-geocode result")
	}
	city := result.Address.City
	if city == "" {
		city = result.Address.Town
	}
	if city == "" {
		city = result.Address.Village
	}
	return &ReversResponse{Address: result.DisplayName, House: result.Address.HouseNumber, Street: result.Address.Road, City: city}, nil
}

func (p *OSMProvider) doJSON(ctx context.Context, requestURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create OSM request: %w", err)
	}
	req.Header.Set("User-Agent", p.userAgent)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("OSM request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("OSM returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(dst); err != nil {
		return fmt.Errorf("decode OSM response: %w", err)
	}
	return nil
}

func parseLatLon(rawLat, rawLon string) (float64, float64, error) {
	lat, err := strconv.ParseFloat(rawLat, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse latitude: %w", err)
	}
	lon, err := strconv.ParseFloat(rawLon, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse longitude: %w", err)
	}
	return lat, lon, nil
}
