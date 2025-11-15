package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Limiter struct {
	mu          sync.RWMutex
	requests    map[string][]time.Time
	maxRequests int
	window      time.Duration
	cleanup     *time.Ticker
	stopCleanup chan struct{}
}

type Config struct {
	MaxRequests int           // Максимальное количество запросов
	Window      time.Duration // Временное окно (например, 1 минута)
	CleanupInt  time.Duration // Интервал очистки старых записей
}

func NewLimiter(cfg Config) *Limiter {
	if cfg.CleanupInt == 0 {
		cfg.CleanupInt = 5 * time.Minute
	}
	l := &Limiter{
		requests:    make(map[string][]time.Time),
		maxRequests: cfg.MaxRequests,
		window:      cfg.Window,
		cleanup:     time.NewTicker(cfg.CleanupInt),
		stopCleanup: make(chan struct{}),
	}
	go l.cleanupLoop()
	return l
}

func (l *Limiter) cleanupLoop() {
	for {
		select {
		case <-l.cleanup.C:
			l.cleanupOld()
		case <-l.stopCleanup:
			return
		}
	}
}

func (l *Limiter) cleanupOld() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	for key, times := range l.requests {
		valid := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.requests, key)
		} else {
			l.requests[key] = valid
		}
	}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Очищаем старые записи для этого ключа
	times := l.requests[key]
	valid := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Проверяем лимит
	if len(valid) >= l.maxRequests {
		return false
	}

	// Добавляем новый запрос
	valid = append(valid, now)
	l.requests[key] = valid
	return true
}

func (l *Limiter) GetIP(r *http.Request) string {
	// Проверяем X-Forwarded-For (для прокси)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Проверяем X-Real-IP
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Используем RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (l *Limiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := l.GetIP(r)
		if !l.Allow("ip:" + ip) {
			http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func (l *Limiter) CheckEmail(email string) bool {
	return l.Allow("email:" + email)
}

func (l *Limiter) Stop() {
	l.cleanup.Stop()
	close(l.stopCleanup)
}

