package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"node-agent/config"
)

func TokenAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-Node-Token")
			if token == "" {
				token = r.URL.Query().Get("token")
			}

			if token != cfg.APIToken {
				writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
					"error": "unauthorized: invalid token",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.CORS.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")
			allowed := false

			if len(cfg.CORS.AllowedOrigins) == 0 {
				allowed = true
			} else {
				for _, o := range cfg.CORS.AllowedOrigins {
					if o == "*" || o == origin {
						allowed = true
						break
					}
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Node-Token, X-Signature, X-Timestamp")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func IPWhitelist(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(cfg.IPWhitelist) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			ip := extractIP(r)
			if !cfg.IsIPWhitelisted(ip) {
				writeJSON(w, http.StatusForbidden, map[string]interface{}{
					"error": "forbidden: ip not whitelisted",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func DeploySignatureVerify(signingKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if signingKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			signature := r.Header.Get("X-Signature")
			timestamp := r.Header.Get("X-Timestamp")
			if signature == "" || timestamp == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
					"error": "missing signature or timestamp",
				})
				return
			}

			ts, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid timestamp format",
				})
				return
			}

			if time.Since(ts) > 5*time.Minute {
				writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
					"error": "request expired",
				})
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{
					"error": "read body failed",
				})
				return
			}

			r.Body = io.NopCloser(strings.NewReader(string(body)))

			message := fmt.Sprintf("%s.%s", timestamp, string(body))
			mac := hmac.New(sha256.New, []byte(signingKey))
			mac.Write([]byte(message))
			expectedSig := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
				writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid signature",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[http] %s %s %v\n", r.Method, r.URL.Path, time.Since(start))
	})
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("[panic] recovered: %v\n", err)
				writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	xfRealIP := r.Header.Get("X-Real-IP")
	if xfRealIP != "" {
		return xfRealIP
	}
	idx := strings.LastIndex(r.RemoteAddr, ":")
	if idx == -1 {
		return r.RemoteAddr
	}
	return r.RemoteAddr[:idx]
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
