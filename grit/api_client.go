package grit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIRequest represents the configuration for an external API call.
type APIRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    interface{}
	Timeout time.Duration
}

// CallAPI is a generic function to make external HTTP requests and decode the response.
func CallAPI(req APIRequest, responseModel interface{}) error {
	var bodyReader io.Reader
	if req.Body != nil {
		jsonData, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	method := req.Method
	if method == "" {
		method = "GET"
	}

	httpReq, err := http.NewRequest(method, req.URL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	timeout := req.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	if responseModel != nil {
		if err := json.NewDecoder(resp.Body).Decode(responseModel); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}
	}

	return nil
}

// --- Common External API Models ---

// WeatherResponse represents a standard weather API response (e.g., OpenWeatherMap).
type WeatherResponse struct {
	Name string `json:"name"`
	Main struct {
		Temp     float64 `json:"temp"`
		Pressure int     `json:"pressure"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Weather []struct {
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
}

// GetWeather is a helper function to fetch weather data.
func GetWeather(apiKey, city string) (*WeatherResponse, error) {
	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", city, apiKey)
	var weather WeatherResponse
	err := CallAPI(APIRequest{URL: url}, &weather)
	return &weather, err
}
