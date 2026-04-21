package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"projectYandexLyceumFinal/internal/handlers"
	"projectYandexLyceumFinal/internal/observability"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	dsn := getEnv("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	port := getEnv("AUTH_PORT", "8081")
	osrmBaseURL := getEnv("OSRM_BASE_URL", "http://osrm:5000")
	geocoderBaseURL := getEnv("GEOCODER_BASE_URL", "https://nominatim.openstreetmap.org")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	metrics := observability.NewCollector()
	observability.SetLogger(logger)
	observability.SetCollector(metrics)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Error("database_open_failed", "service", "auth-service", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Error("database_ping_failed", "service", "auth-service", "error", err)
		os.Exit(1)
	}

	handlers.SetDB(db)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(observability.Middleware())
	r.Use(corsMiddleware())
	r.GET("/metrics", gin.WrapH(observability.Handler()))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/api/route", makeRouteHandler(osrmBaseURL))
	r.GET("/api/geocode", makeGeocodeHandler(geocoderBaseURL))
	r.GET("/api/geocode/suggest", makeGeocodeSuggestHandler(geocoderBaseURL))
	r.POST("/api/auth/register", handlers.Register)
	r.POST("/api/auth/login", handlers.Login)

	logger.Info("auth_service_listening", "service", "auth-service", "port", port)
	if err := r.Run(":" + port); err != nil {
		logger.Error("server_failed", "service", "auth-service", "error", err)
		os.Exit(1)
	}

}

func makeGeocodeSuggestHandler(geocoderBaseURL string) gin.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}

	return func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("query"))
		if len([]rune(query)) < 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query must contain at least 3 characters"})
			return
		}

		baseURL := strings.TrimRight(geocoderBaseURL, "/")
		queryVariants := []string{query}
		lower := strings.ToLower(query)
		if strings.HasPrefix(lower, "ул ") {
			queryVariants = append(queryVariants, "улица "+strings.TrimSpace(query[4:]))
		}
		if strings.HasPrefix(lower, "ул. ") {
			queryVariants = append(queryVariants, "улица "+strings.TrimSpace(query[5:]))
		}
		if strings.HasPrefix(lower, "пр ") || strings.HasPrefix(lower, "пр. ") {
			parts := strings.Fields(query)
			if len(parts) > 1 {
				queryVariants = append(queryVariants, "проспект "+strings.Join(parts[1:], " "))
			}
		}
		if !strings.Contains(lower, "москва") && !strings.Contains(lower, "moscow") {
			queryVariants = append(queryVariants, query+", Москва")
			queryVariants = append(queryVariants, query+", Москва, Россия")
		}

		suggestions := make([]gin.H, 0, 8)
		seen := make(map[string]struct{})

		for _, variant := range queryVariants {
			params := url.Values{}
			params.Set("format", "json")
			params.Set("limit", "5")
			params.Set("countrycodes", "ru")
			params.Set("q", variant)
			params.Set("viewbox", "36.80,56.10,38.40,55.10")
			params.Set("bounded", "0")

			requestURL := fmt.Sprintf("%s/search?%s", baseURL, params.Encode())
			req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, requestURL, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", "delivery-service/1.0")
			req.Header.Set("Accept-Language", "ru")

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusOK {
				continue
			}

			var items []struct {
				Lat         string `json:"lat"`
				Lon         string `json:"lon"`
				DisplayName string `json:"display_name"`
			}

			if err := json.Unmarshal(body, &items); err != nil {
				continue
			}

			for _, item := range items {
				lat, err := strconv.ParseFloat(item.Lat, 64)
				if err != nil {
					continue
				}
				lon, err := strconv.ParseFloat(item.Lon, 64)
				if err != nil {
					continue
				}

				key := fmt.Sprintf("%0.6f:%0.6f", lat, lon)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}

				suggestions = append(suggestions, gin.H{
					"display_name": item.DisplayName,
					"lat":          lat,
					"lon":          lon,
				})

				if len(suggestions) >= 8 {
					c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
					return
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
	}
}

func makeGeocodeHandler(geocoderBaseURL string) gin.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}

	return func(c *gin.Context) {
		address := strings.TrimSpace(c.Query("address"))
		if address == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing query parameter: address"})
			return
		}

		baseURL := strings.TrimRight(geocoderBaseURL, "/")
		queries := []string{address}
		lowerAddress := strings.ToLower(address)
		if !strings.Contains(lowerAddress, "москва") && !strings.Contains(lowerAddress, "moscow") {
			queries = append(queries, address+", Москва")
			queries = append(queries, address+", Москва, Россия")
		}

		for _, query := range queries {
			params := url.Values{}
			params.Set("format", "json")
			params.Set("limit", "1")
			params.Set("countrycodes", "ru")
			params.Set("q", query)
			params.Set("viewbox", "36.80,56.10,38.40,55.10")
			params.Set("bounded", "0")

			requestURL := fmt.Sprintf("%s/search?%s", baseURL, params.Encode())

			req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, requestURL, nil)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare geocoder request"})
				return
			}
			req.Header.Set("User-Agent", "delivery-service/1.0")
			req.Header.Set("Accept-Language", "ru")

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				continue
			}

			if resp.StatusCode != http.StatusOK {
				continue
			}

			var items []struct {
				Lat         string `json:"lat"`
				Lon         string `json:"lon"`
				DisplayName string `json:"display_name"`
			}

			if err := json.Unmarshal(body, &items); err != nil {
				continue
			}

			if len(items) == 0 {
				continue
			}

			lat, err := strconv.ParseFloat(items[0].Lat, 64)
			if err != nil {
				continue
			}
			lon, err := strconv.ParseFloat(items[0].Lon, 64)
			if err != nil {
				continue
			}

			c.JSON(http.StatusOK, gin.H{
				"lat":          lat,
				"lon":          lon,
				"display_name": items[0].DisplayName,
			})
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
	}
}

func makeRouteHandler(osrmBaseURL string) gin.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}

	return func(c *gin.Context) {
		fromLat, err := parseFloatQuery(c, "fromLat")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fromLon, err := parseFloatQuery(c, "fromLon")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		toLat, err := parseFloatQuery(c, "toLat")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		toLon, err := parseFloatQuery(c, "toLon")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		candidateBaseURLs := []string{strings.TrimRight(osrmBaseURL, "/"), "https://router.project-osrm.org"}
		seenBaseURLs := map[string]struct{}{}

		for _, baseURL := range candidateBaseURLs {
			if baseURL == "" {
				continue
			}
			if _, ok := seenBaseURLs[baseURL]; ok {
				continue
			}
			seenBaseURLs[baseURL] = struct{}{}

			route, err := fetchRouteFromOSRM(c.Request.Context(), client, baseURL, fromLat, fromLon, toLat, toLon)
			if err == nil {
				c.JSON(http.StatusOK, gin.H{
					"distance": route.Distance,
					"duration": route.Duration,
					"geometry": route.Geometry,
				})
				return
			}

			observability.Logger().Warn("route_osrm_candidate_failed", "base_url", baseURL, "error", err)
		}

		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to call OSRM"})
	}
}

func fetchRouteFromOSRM(ctx context.Context, client *http.Client, baseURL string, fromLat, fromLon, toLat, toLon float64) (*struct {
	Distance float64
	Duration float64
	Geometry struct {
		Coordinates [][]float64 `json:"coordinates"`
	}
}, error) {
	requestURL := fmt.Sprintf(
		"%s/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		baseURL,
		fromLon,
		fromLat,
		toLon,
		toLat,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSRM returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var osrmResp struct {
		Routes []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}

	if err := json.Unmarshal(body, &osrmResp); err != nil {
		return nil, err
	}

	if len(osrmResp.Routes) == 0 {
		return nil, fmt.Errorf("route not found")
	}

	route := osrmResp.Routes[0]
	return &struct {
		Distance float64
		Duration float64
		Geometry struct {
			Coordinates [][]float64 `json:"coordinates"`
		}
	}{
		Distance: route.Distance,
		Duration: route.Duration,
		Geometry: route.Geometry,
	}, nil
}

func parseFloatQuery(c *gin.Context, key string) (float64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, fmt.Errorf("missing query parameter: %s", key)
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid query parameter %s", key)
	}

	return value, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}

}
