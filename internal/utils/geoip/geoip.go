package geoip

import (
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/uSipipo-Team/usipipo-agent/internal/config"
)

// GeoIPResponse represents the response from ip-api.com
type GeoIPResponse struct {
	Query       string `json:"query"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
	RegionName  string `json:"regionName"`
	City        string `json:"city"`
}

// GetLocation fetches public IP and geo location using HTTPS with retry logic
// If GeoIP is disabled or fails after retries, returns default values (graceful degradation)
func GetLocation(cfg *config.Config) (*GeoIPResponse, error) {
	// If GeoIP is disabled, return defaults immediately
	if !cfg.GeoIPEnabled {
		return &GeoIPResponse{
			Query:       "disabled",
			CountryCode: "XX",
			CountryName: "GeoIP Disabled",
			RegionName:  "Unknown",
			City:        "Unknown",
		}, nil
	}

	// Create dedicated client with configurable settings
	geoClient := resty.New()
	geoClient.SetTimeout(cfg.GeoIPTimeout)
	geoClient.SetRetryCount(cfg.GeoIPMaxRetries)
	// Use exponential backoff: base wait time
	geoClient.SetRetryWaitTime(cfg.GeoIPRetryBackoff)
	geoClient.SetRetryMaxWaitTime(cfg.GeoIPRetryBackoff * 16) // Cap at 16x base

	var resp *resty.Response
	var err error

	// Execute with retry logic (resty handles retries automatically)
	resp, err = geoClient.R().
		Get("https://ip-api.com/json/")

	if err != nil {
		return nil, fmt.Errorf("failed to fetch geo location after %d retries: %w", cfg.GeoIPMaxRetries, err)
	}

	if resp == nil || resp.StatusCode() != 200 {
		return nil, fmt.Errorf("geo API returned status %d after %d retries",
			func() int {
				if resp != nil {
					return resp.StatusCode()
				}
				return 0
			}(),
			cfg.GeoIPMaxRetries)
	}

	var geo GeoIPResponse
	if err := json.Unmarshal(resp.Body(), &geo); err != nil {
		return nil, fmt.Errorf("failed to parse geo response: %w", err)
	}

	return &geo, nil
}
