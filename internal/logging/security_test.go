package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"valid key", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7", "agen...p6q7"},
		{"valid key uppercase", "agent_A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7", "agen...P6Q7"},
		{"short key", "agent_short", "***"},
		{"empty key", "", "***"},
		{"single char", "a", "***"},
		{"exactly 8 chars", "12345678", "1234...5678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskAPIKey(tt.key)
			if result != tt.expected {
				t.Errorf("maskAPIKey(%q) = %q, expected %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal string", "Hello World", "Hello World"},
		{"bearer token", "Bearer secret123", "Bearer ***"},
		{"long string", strings.Repeat("a", 1500), strings.Repeat("a", 1000) + "..."},
		{"API key in string", "Key: agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7", "Key: agen...p6q7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeString(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeHeaders(t *testing.T) {
	headers := map[string][]string{
		"Content-Type":  {"application/json"},
		"User-Agent":    {"Mozilla/5.0"},
		"Authorization": {"Bearer secret"},
		"X-API-Key":     {"agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7"},
		"Cookie":        {"session=secret123"},
		"Accept":        {"*/*"},
	}

	safe := sanitizeHeaders(headers)

	// Should include safe headers
	if safe["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type to be preserved")
	}
	if safe["User-Agent"] != "Mozilla/5.0" {
		t.Errorf("Expected User-Agent to be preserved")
	}
	if safe["Accept"] != "*/*" {
		t.Errorf("Expected Accept to be preserved")
	}

	// Should exclude sensitive headers
	if _, exists := safe["Authorization"]; exists {
		t.Errorf("Authorization header should be excluded")
	}
	if _, exists := safe["X-API-Key"]; exists {
		t.Errorf("X-API-Key header should be excluded")
	}
	if _, exists := safe["Cookie"]; exists {
		t.Errorf("Cookie header should be excluded")
	}
}

func TestContainsSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"normal string", "Hello World", false},
		{"API key pattern", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7", true},
		{"bearer token", "Bearer secret", true},
		{"contains agent_", "This has agent_ in it", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSensitiveData(tt.input)
			if result != tt.expected {
				t.Errorf("containsSensitiveData(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSecurityLogger_LogAuthFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		out:      &buf,
		level:    InfoLevel,
		serverID: "test-server",
	}

	logger.LogAuthFailure("192.168.1.1", "/api/v1/status", "invalid_key", "Mozilla/5.0")

	// Parse JSON output
	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	// Verify fields
	if entry.Event != AuthFailureEvent {
		t.Errorf("Expected event %s, got %s", AuthFailureEvent, entry.Event)
	}
	if entry.Level != WarnLevel {
		t.Errorf("Expected level %s, got %s", WarnLevel, entry.Level)
	}
	if entry.IP != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", entry.IP)
	}
	if entry.ServerID != "test-server" {
		t.Errorf("Expected serverID test-server, got %s", entry.ServerID)
	}

	// Verify no sensitive data in raw output
	rawLog := buf.String()
	if strings.Contains(rawLog, "agent_") && !strings.Contains(rawLog, "...") {
		t.Errorf("Log output may contain unmasked API keys: %s", rawLog)
	}
}

func TestSecurityLogger_LogRateLimitExceeded(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		out:      &buf,
		level:    InfoLevel,
		serverID: "test-server",
	}

	logger.LogRateLimitExceeded("10.0.0.1", "/api/v1/status", 5.0)

	// Parse JSON output
	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	// Verify fields
	if entry.Event != RateLimitExceededEvent {
		t.Errorf("Expected event %s, got %s", RateLimitExceededEvent, entry.Event)
	}
	if entry.Level != WarnLevel {
		t.Errorf("Expected level %s, got %s", WarnLevel, entry.Level)
	}
}

func TestSecurityLogger_ShouldLog(t *testing.T) {
	tests := []struct {
		name     string
		logLevel LogLevel
		testLevel LogLevel
		expected bool
	}{
		{"INFO logger logs INFO", InfoLevel, InfoLevel, true},
		{"INFO logger logs WARN", InfoLevel, WarnLevel, true},
		{"INFO logger logs ERROR", InfoLevel, ErrorLevel, true},
		{"WARN logger doesn't log INFO", WarnLevel, InfoLevel, false},
		{"WARN logger logs WARN", WarnLevel, WarnLevel, true},
		{"WARN logger logs ERROR", WarnLevel, ErrorLevel, true},
		{"ERROR logger only logs ERROR", ErrorLevel, InfoLevel, false},
		{"ERROR logger logs ERROR", ErrorLevel, ErrorLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &SecurityLogger{
				level: tt.logLevel,
			}
			result := logger.shouldLog(tt.testLevel)
			if result != tt.expected {
				t.Errorf("shouldLog(%s) with logger level %s = %v, expected %v",
					tt.testLevel, tt.logLevel, result, tt.expected)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected LogLevel
	}{
		{"INFO", "INFO", InfoLevel},
		{"WARN", "WARN", WarnLevel},
		{"ERROR", "ERROR", ErrorLevel},
		{"invalid", "invalid", InfoLevel},
		{"empty", "", InfoLevel},
		{"lowercase info", "info", InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSecurityLogger_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		out:      &buf,
		level:    InfoLevel,
		serverID: "test-server",
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.LogAuthFailure(
				fmt.Sprintf("192.168.1.%d", id),
				"/api/v1/status",
				"test",
				"Mozilla/5.0",
			)
		}(i)
	}

	wg.Wait()

	// Verify all logs were written without panic
	lines := strings.Count(buf.String(), "\n")
	if lines != 100 {
		t.Errorf("Expected 100 log lines, got %d", lines)
	}
}

func TestSecurityLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := &SecurityLogger{
		out:      &buf,
		level:    WarnLevel,
		serverID: "test-server",
	}

	logger.LogStartup("test") // INFO level - should not be logged
	if buf.Len() > 0 {
		t.Errorf("Expected no output for INFO level with WARN logger, got: %s", buf.String())
	}

	buf.Reset()
	logger.LogAuthFailure("192.168.1.1", "/api", "test", "Mozilla") // WARN level - should be logged
	if buf.Len() == 0 {
		t.Error("Expected WARN level log to be written")
	}
}
