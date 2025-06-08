package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ClientRateLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiterMiddleware struct {
	clients    map[string]*ClientRateLimiter
	mu         sync.Mutex
	rate       rate.Limit
	burst      int
	expiration time.Duration
	logger     *slog.Logger

	ctx context.Context
}

func NewRateLimiterMiddleware(
	ctx context.Context,
	requestsPerSecond int,
	window time.Duration,
	logger *slog.Logger,
) *RateLimiterMiddleware {
	r := rate.Limit(float64(requestsPerSecond) / window.Seconds())

	m := &RateLimiterMiddleware{
		clients:    make(map[string]*ClientRateLimiter),
		rate:       r,
		burst:      requestsPerSecond,
		expiration: 1 * time.Hour,
		logger:     logger,
		ctx:        ctx,
	}

	go m.cleanupClients()

	return m
}

func (m *RateLimiterMiddleware) getClientLimiter(ip string) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[ip]
	if !exists {
		client = &ClientRateLimiter{
			limiter:  rate.NewLimiter(m.rate, m.burst),
			lastSeen: time.Now(),
		}
		m.clients[ip] = client
	} else {
		client.lastSeen = time.Now()
	}

	return client.limiter
}

func (m *RateLimiterMiddleware) cleanupClients() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			for ip, client := range m.clients {
				if time.Since(client.lastSeen) > m.expiration {
					delete(m.clients, ip)
				}
			}
			m.mu.Unlock()
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *RateLimiterMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		limiter := m.getClientLimiter(ip)

		if !limiter.Allow() {
			retryAfter := int(1 / float64(m.rate))
			if retryAfter < 1 {
				retryAfter = 1
			}

			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", m.burst))
			w.Header().Set("X-RateLimit-Remaining", "0")

			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)

			return
		}

		next.ServeHTTP(w, r)
	})
}
