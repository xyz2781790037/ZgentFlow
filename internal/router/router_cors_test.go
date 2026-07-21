package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSOriginAllowed(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://trusted.example")

	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "LAN same origin", host: "10.30.0.131:5173", origin: "http://10.30.0.131:5173", want: true},
		{name: "different port", host: "10.30.0.131:8081", origin: "http://10.30.0.131:5173", want: false},
		{name: "configured origin", host: "127.0.0.1:8081", origin: "https://trusted.example", want: true},
		{name: "invalid origin", host: "10.30.0.131:5173", origin: "://invalid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "http://"+tt.host+"/", nil)
			if got := corsOriginAllowed(c, tt.origin); got != tt.want {
				t.Fatalf("corsOriginAllowed(%q, host %q) = %v, want %v", tt.origin, tt.host, got, tt.want)
			}
		})
	}
}
