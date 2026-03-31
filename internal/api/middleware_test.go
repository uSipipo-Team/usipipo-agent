package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyMiddleware(t *testing.T) {
	validKey := "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
	router := gin.New()
	router.Use(APIKeyMiddleware(validKey))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		apiKey         string
		expectedStatus int
	}{
		{"valid key", validKey, http.StatusOK},
		{"missing key", "", http.StatusUnauthorized},
		{"wrong key", "agent_x1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", http.StatusUnauthorized},
		{"malformed key", "invalid-key", http.StatusUnauthorized},
		{"too short", "agent_short", http.StatusUnauthorized},
		{"no prefix", "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
