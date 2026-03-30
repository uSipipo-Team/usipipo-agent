package geoip

import (
	"encoding/json"
	"fmt"

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

// GetLocation fetches public IP and geo location
func GetLocation(client *resty.Client) (*GeoIPResponse, error) {
	resp, err := client.R().
		Get("http://ip-api.com/json/")

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
