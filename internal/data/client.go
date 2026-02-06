package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

type openMeteoResponse struct {
	Hourly struct {
		SurfacePressure []float64 `json:"surface_pressure"`
	} `json:"hourly"`
}

func (c *Client) FetchPressureDelta(ctx context.Context, lat, lon float64) (float64, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.5f", lat))
	q.Set("longitude", fmt.Sprintf("%.5f", lon))
	q.Set("hourly", "surface_pressure")
	q.Set("forecast_days", "2")
	q.Set("timezone", "auto")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("open-meteo: unexpected status %d", resp.StatusCode)
	}

	var parsed openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	if len(parsed.Hourly.SurfacePressure) < 24 {
		return 0, fmt.Errorf("open-meteo: not enough hourly pressure points: %d", len(parsed.Hourly.SurfacePressure))
	}

	minV := parsed.Hourly.SurfacePressure[0]
	maxV := parsed.Hourly.SurfacePressure[0]
	for _, value := range parsed.Hourly.SurfacePressure[:24] {
		if value < minV {
			minV = value
		}
		if value > maxV {
			maxV = value
		}
	}
	return maxV - minV, nil
}

func (c *Client) FetchLatestKIndex(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://services.swpc.noaa.gov/json/planetary_k_index_1m.json",
		nil,
	)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("noaa: unexpected status %d", resp.StatusCode)
	}

	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("noaa: empty response")
	}

	last := rows[len(rows)-1]
	for _, key := range []string{"k_index", "kp_index", "kp", "kIndex"} {
		if raw, ok := last[key]; ok {
			value, ok := anyToFloat(raw)
			if ok {
				return value, nil
			}
		}
	}

	return 0, fmt.Errorf("noaa: k-index field not found")
}

func anyToFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		val, err := x.Float64()
		if err != nil {
			return 0, false
		}
		return val, true
	case string:
		val, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, false
		}
		return val, true
	default:
		return 0, false
	}
}
