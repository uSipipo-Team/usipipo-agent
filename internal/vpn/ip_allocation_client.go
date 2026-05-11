package vpn

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	ErrReserveFailed = errors.New("ip reserve failed")
	ErrConfirmFailed = errors.New("ip confirm failed")
	ErrReleaseFailed = errors.New("ip release failed")
	ErrServerError   = errors.New("server error")
)

const (
	defaultTimeout = 15 * time.Second
	maxRetries     = 3
	baseRetryDelay = 100 * time.Millisecond
)

type IPReserveRequest struct {
	ServerID   string `json:"server_id"`
	VpnKeyName string `json:"vpn_key_name"`
	VpnType    string `json:"vpn_type"`
	RequestID  string `json:"request_id"`
}

type IPReserveResponse struct {
	IPAddress      string    `json:"ip_address"`
	IPInt          int       `json:"ip_int"`
	LeaseID        string    `json:"lease_id"`
	CIDR           string    `json:"cidr"`
	PublicKey      string    `json:"public_key"`
	PrivateKey     string    `json:"private_key"`
	PresharedKey   string    `json:"preshared_key"`
	Config         string    `json:"config"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

type IPConfirmRequest struct {
	LeaseID   string `json:"lease_id"`
	IPAddress string `json:"ip_address"`
	PublicKey string `json:"public_key"`
	RequestID string `json:"request_id"`
}

type IPReleaseRequest struct {
	LeaseID   string `json:"lease_id"`
	Reason    string `json:"reason"`
	RequestID string `json:"request_id"`
}

type IPAllocationClient struct {
	baseURL    string
	apiKey     string
	serverID   string
	httpClient *http.Client
	logger     *log.Logger
	timeout    time.Duration
}

func NewIPAllocationClient(baseURL, apiKey, serverID string, logger *log.Logger) *IPAllocationClient {
	if logger == nil {
		logger = log.New(log.Writer(), "[IPAlloc] ", log.Flags())
	}
	return &IPAllocationClient{
		baseURL:  baseURL,
		apiKey:   apiKey,
		serverID: serverID,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		logger:  logger,
		timeout: defaultTimeout,
	}
}

func (c *IPAllocationClient) logInfo(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Print("INFO: ", msg)
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				c.logger.Print(" ", keysAndValues[i], "=", keysAndValues[i+1])
			}
		}
	}
}

func (c *IPAllocationClient) logWarn(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Print("WARN: ", msg)
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				c.logger.Print(" ", keysAndValues[i], "=", keysAndValues[i+1])
			}
		}
	}
}

func (c *IPAllocationClient) logError(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Print("ERROR: ", msg)
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				c.logger.Print(" ", keysAndValues[i], "=", keysAndValues[i+1])
			}
		}
	}
}

func (c *IPAllocationClient) computeSignature(body []byte, timestamp int64) string {
	h := hmac.New(sha256.New, []byte(c.apiKey))
	h.Write(body)
	io.WriteString(h, fmt.Sprint(timestamp))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (c *IPAllocationClient) doRequestWithRetry(
	ctx context.Context,
	method string,
	path string,
	reqBody interface{},
	respBody interface{},
) error {
	var bodyBytes []byte
	var err error

	if reqBody != nil {
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
	}

	var lastErr error
	retryDelays := []time.Duration{
		baseRetryDelay,
		baseRetryDelay * 2,
		baseRetryDelay * 4,
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelays[attempt-1]
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		lastErr = c.doRequest(ctx, method, path, bodyBytes, respBody)
		if lastErr == nil {
			return nil
		}

		if shouldRetry(lastErr) {
			c.logWarn("request_retry",
				"method", method,
				"path", path,
				"attempt", attempt+1,
				"error", lastErr)
			continue
		}

		break
	}

	return lastErr
}

func (c *IPAllocationClient) doRequest(
	ctx context.Context,
	method string,
	path string,
	bodyBytes []byte,
	respBody interface{},
) error {
	url := c.baseURL + path
	timestamp := time.Now().Unix()
	signature := c.computeSignature(bodyBytes, timestamp)

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("X-Timestamp", fmt.Sprint(timestamp))
	req.Header.Set("X-Signature", signature)

	c.logInfo("request_started",
		"method", method,
		"path", path,
		"url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logError("request_failed",
			"method", method,
			"path", path,
			"error", err)
		return fmt.Errorf("%w: %v", ErrServerError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found: %s", resp.Status)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("%w: rate limited", ErrServerError)
	}

	if resp.StatusCode >= 500 {
		c.logError("server_error",
			"method", method,
			"path", path,
			"status", resp.StatusCode)
		return fmt.Errorf("%w: status=%d", ErrServerError, resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		c.logError("client_error",
			"method", method,
			"path", path,
			"status", resp.StatusCode)
		return fmt.Errorf("%w: status=%d", ErrReserveFailed, resp.StatusCode)
	}

	if respBody != nil && len(bodyBytes) > 0 {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			c.logError("decode_failed",
				"method", method,
				"path", path,
				"error", err)
			return fmt.Errorf("decode response: %w", err)
		}
	}

	c.logInfo("request_completed",
		"method", method,
		"path", path,
		"status", resp.StatusCode)

	return nil
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, ErrServerError.Error()) ||
		strings.Contains(errStr, "rate limited") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout")
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "404")
}

func (c *IPAllocationClient) ReserveIP(ctx context.Context, keyName string) (*IPReserveResponse, error) {
	req := IPReserveRequest{
		ServerID:   c.serverID,
		VpnKeyName: keyName,
		VpnType:    "wireguard",
		RequestID:  generateRequestID(),
	}

	var resp IPReserveResponse
	err := c.doRequestWithRetry(ctx, "POST", "/wireguard/reserve-ip", req, &resp)
	if err != nil {
		c.logError("reserve_failed",
			"key_name", keyName,
			"error", err)
		return nil, fmt.Errorf("%w: %v", ErrReserveFailed, err)
	}

	c.logInfo("ip_reserved",
		"key_name", keyName,
		"ip_address", resp.IPAddress,
		"lease_id", resp.LeaseID)

	return &resp, nil
}

func (c *IPAllocationClient) ConfirmAllocation(ctx context.Context, leaseID, ipAddress, publicKey string) error {
	req := IPConfirmRequest{
		LeaseID:   leaseID,
		IPAddress: ipAddress,
		PublicKey: publicKey,
		RequestID: generateRequestID(),
	}

	err := c.doRequestWithRetry(ctx, "POST", "/wireguard/confirm-allocation", req, nil)
	if err != nil {
		if isNotFound(err) {
			c.logWarn("confirm_not_found",
				"lease_id", leaseID,
				"ip_address", ipAddress)
			return nil
		}
		c.logError("confirm_failed",
			"lease_id", leaseID,
			"ip_address", ipAddress,
			"error", err)
		return fmt.Errorf("%w: %v", ErrConfirmFailed, err)
	}

	c.logInfo("allocation_confirmed",
		"lease_id", leaseID,
		"ip_address", ipAddress)

	return nil
}

func (c *IPAllocationClient) ReleaseIP(ctx context.Context, leaseID, reason string) error {
	req := IPReleaseRequest{
		LeaseID:   leaseID,
		Reason:    reason,
		RequestID: generateRequestID(),
	}

	err := c.doRequestWithRetry(ctx, "POST", "/wireguard/release-ip", req, nil)
	if err != nil {
		if isNotFound(err) {
			c.logWarn("release_not_found",
				"lease_id", leaseID,
				"reason", reason)
			return nil
		}
		c.logError("release_failed",
			"lease_id", leaseID,
			"reason", reason,
			"error", err)
		return fmt.Errorf("%w: %v", ErrReleaseFailed, err)
	}

	c.logInfo("ip_released",
		"lease_id", leaseID,
		"reason", reason)

	return nil
}

func generateRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%x", time.Now().UnixNano(), b)
}
