package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// loginAttempt tracks failed login attempts per IP
type loginAttempt struct {
	count     int
	firstSeen time.Time
}

var (
	loginLimiterMu sync.Mutex
	loginLimiter   = make(map[string]*loginAttempt)
)

const (
	maxAttempts    = 10
	blockDuration  = 15 * time.Minute
	cleanupInterval = 30 * time.Minute
)

func init() {
	go func() {
		for {
			time.Sleep(cleanupInterval)
			loginLimiterMu.Lock()
			now := time.Now()
			for ip, a := range loginLimiter {
				if now.Sub(a.firstSeen) > blockDuration {
					delete(loginLimiter, ip)
				}
			}
			loginLimiterMu.Unlock()
		}
	}()
}

// LoginRateLimit returns a middleware that rate-limits POST requests to login endpoints.
func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only throttle POST to login paths
		if c.Request.Method != "POST" {
			c.Next()
			return
		}

		ip := c.ClientIP()

		loginLimiterMu.Lock()
		attempt := loginLimiter[ip]
		now := time.Now()

		if attempt != nil && now.Sub(attempt.firstSeen) > blockDuration {
			// Reset after block duration
			delete(loginLimiter, ip)
			attempt = nil
		}

		if attempt == nil {
			attempt = &loginAttempt{count: 0, firstSeen: now}
			loginLimiter[ip] = attempt
		}

		blocked := attempt.count >= maxAttempts
		loginLimiterMu.Unlock()

		if blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "登录尝试过于频繁，请15分钟后再试",
			})
			c.Abort()
			return
		}

		// Track after handler runs (only count failures)
		c.Next()

		if c.Writer.Status() == http.StatusUnauthorized {
			loginLimiterMu.Lock()
			if a := loginLimiter[ip]; a != nil {
				a.count++
			}
			loginLimiterMu.Unlock()
		}
	}
}
