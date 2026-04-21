package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"projectYandexLyceumFinal/internal/models"
)

var orderServiceBaseURL = "http://order-service:8080"

func SetOrderServiceURL(baseURL string) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		orderServiceBaseURL = strings.TrimRight(baseURL, "/")
	}
}

func registerCourierProfile(ctx context.Context, input models.RegisterInput) (string, error) {
	payload := map[string]string{
		"email":          input.Email,
		"full_name":      input.Name,
		"phone":          input.PhoneNumber,
		"transport_type": strings.TrimSpace(input.TransportType),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, orderServiceBaseURL+"/api/v1/couriers/register", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		var responseBody bytes.Buffer
		_, _ = responseBody.ReadFrom(response.Body)
		return "", fmt.Errorf("order-service courier register failed: %s", strings.TrimSpace(responseBody.String()))
	}

	var decoded struct {
		CourierID string `json:"courier_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return "", err
	}

	if decoded.CourierID == "" {
		return "", fmt.Errorf("order-service returned empty courier_id")
	}

	return decoded.CourierID, nil
}

func lookupCourierIDByEmail(ctx context.Context, email string) (string, error) {
	requestURL := orderServiceBaseURL + "/api/v1/couriers/by-email?" + url.Values{"email": {email}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		var responseBody bytes.Buffer
		_, _ = responseBody.ReadFrom(response.Body)
		return "", fmt.Errorf("order-service courier lookup failed: %s", strings.TrimSpace(responseBody.String()))
	}

	var decoded struct {
		CourierID string `json:"courier_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return "", err
	}

	if decoded.CourierID == "" {
		return "", fmt.Errorf("order-service returned empty courier_id")
	}

	return decoded.CourierID, nil
}