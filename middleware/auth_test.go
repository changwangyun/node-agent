package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"node-agent/config"
)

func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}

func TestTokenAuthValid(t *testing.T) {
	cfg := &config.Config{APIToken: "test-token"}
	handler := TokenAuth(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Node-Token", "test-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTokenAuthInvalid(t *testing.T) {
	cfg := &config.Config{APIToken: "test-token"}
	handler := TokenAuth(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Node-Token", "wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTokenAuthQueryToken(t *testing.T) {
	cfg := &config.Config{APIToken: "test-token"}
	handler := TokenAuth(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test?token=test-token", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTokenAuthMissing(t *testing.T) {
	cfg := &config.Config{APIToken: "test-token"}
	handler := TokenAuth(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIPWhitelistEmpty(t *testing.T) {
	cfg := &config.Config{IPWhitelist: []string{}}
	handler := IPWhitelist(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty whitelist should allow all, got %d", w.Code)
	}
}

func TestIPWhitelistAllowed(t *testing.T) {
	cfg := &config.Config{IPWhitelist: []string{"10.0.0.1"}}
	handler := IPWhitelist(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestIPWhitelistBlocked(t *testing.T) {
	cfg := &config.Config{IPWhitelist: []string{"10.0.0.1"}}
	handler := IPWhitelist(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestIPWhitelistXForwardedFor(t *testing.T) {
	cfg := &config.Config{IPWhitelist: []string{"10.0.0.1"}}
	handler := IPWhitelist(cfg)(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for X-Forwarded-For IP, got %d", w.Code)
	}
}

func TestDeploySignatureVerifyNoKey(t *testing.T) {
	handler := DeploySignatureVerify("")(testHandler())

	req := httptest.NewRequest("POST", "/deploy", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty signing key should skip verification, got %d", w.Code)
	}
}

func TestDeploySignatureVerifyNonPost(t *testing.T) {
	handler := DeploySignatureVerify("my-secret")(testHandler())

	req := httptest.NewRequest("GET", "/deploy", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("non-POST should skip signature verification, got %d", w.Code)
	}
}

func TestDeploySignatureVerifyValid(t *testing.T) {
	signingKey := "my-secret-key"
	handler := DeploySignatureVerify(signingKey)(testHandler())

	body := `{"user_id":"u1","protocol":"hysteria2"}`
	timestamp := time.Now().Format(time.RFC3339)
	message := fmt.Sprintf("%s.%s", timestamp, body)
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/deploy", strings.NewReader(body))
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", timestamp)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid signature, got %d", w.Code)
	}
}

func TestDeploySignatureVerifyInvalid(t *testing.T) {
	signingKey := "my-secret-key"
	handler := DeploySignatureVerify(signingKey)(testHandler())

	body := `{"user_id":"u1","protocol":"hysteria2"}`
	timestamp := time.Now().Format(time.RFC3339)

	req := httptest.NewRequest("POST", "/deploy", strings.NewReader(body))
	req.Header.Set("X-Signature", "invalid-signature")
	req.Header.Set("X-Timestamp", timestamp)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid signature, got %d", w.Code)
	}
}

func TestDeploySignatureVerifyMissingHeaders(t *testing.T) {
	signingKey := "my-secret-key"
	handler := DeploySignatureVerify(signingKey)(testHandler())

	req := httptest.NewRequest("POST", "/deploy", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing headers, got %d", w.Code)
	}
}

func TestDeploySignatureVerifyExpired(t *testing.T) {
	signingKey := "my-secret-key"
	handler := DeploySignatureVerify(signingKey)(testHandler())

	body := `{"user_id":"u1"}`
	timestamp := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	message := fmt.Sprintf("%s.%s", timestamp, body)
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/deploy", strings.NewReader(body))
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", timestamp)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired request, got %d", w.Code)
	}
}

func TestRecovery(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := Recovery(panicHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic recovery, got %d", w.Code)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		realIP     string
		expected   string
	}{
		{"direct", "10.0.0.1:12345", "", "", "10.0.0.1"},
		{"xff", "1.1.1.1:80", "10.0.0.2, 192.168.1.1", "", "10.0.0.2"},
		{"real_ip", "1.1.1.1:80", "", "10.0.0.3", "10.0.0.3"},
		{"xff_priority", "1.1.1.1:80", "10.0.0.4", "10.0.0.5", "10.0.0.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}

			ip := extractIP(req)
			if ip != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, ip)
			}
		})
	}
}
