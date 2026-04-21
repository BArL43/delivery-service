package geocoder

import "context"

type Coordinates struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

type AddressSuggestion struct {
	DisplayName string  `json:"displayName"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Confidence  float64 `json:"confidence"`
}

type ReversResponse struct {
	Address string `json:"address"`
	House   string `json:"houseNumber"`
	Street  string `json:"street"`
	City    string `json:"city"`
}

type GeocodeProvider interface {
	Geocode(ctx context.Context, address string) (*Coordinates, error)
	Name() string
	Suggest(ctx context.Context, query string, limit int) ([]AddressSuggestion, error)
	ReverseGeocode(ctx context.Context, lat float64, lon float64) (*ReversResponse, error)
}

type GeocodeCache interface {
	Get(ctx context.Context, key string) (*Coordinates, error)
	Set(ctx context.Context, key string, coords *Coordinates) error
}
