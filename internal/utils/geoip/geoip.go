package geoip

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// GeoIPResponse represents the response from ip-api.com
type GeoIPResponse struct {
	Query       string `json:"query"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
	RegionName  string `json:"regionName"`
	City        string `json:"city"`
}

// GetLocation fetches public IP and geo location using HTTPS
func GetLocation(client *resty.Client) (*GeoIPResponse, error) {
	// Create dedicated client to avoid modifying shared instance
	geoClient := resty.New()
	geoClient.SetTimeout(10 * time.Second)
	geoClient.SetRetryCount(2)
	geoClient.SetRetryWaitTime(1 * time.Second)
	geoClient.SetRetryMaxWaitTime(5 * time.Second)

	resp, err := geoClient.R().
		Get("https://ip-api.com/json/") // HTTPS for security

	if err != nil {
		return nil, fmt.Errorf("failed to fetch geo location: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("geo API returned status %d", resp.StatusCode())
	}

	var geo GeoIPResponse
	if err := json.Unmarshal(resp.Body(), &geo); err != nil {
		return nil, fmt.Errorf("failed to parse geo response: %w", err)
	}

	return &geo, nil
}
