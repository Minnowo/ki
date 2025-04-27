package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limters      map[string]*rate.Limiter
	limitersLock sync.RWMutex
	rate         rate.Limit
	burst        int
}

func New(limit rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{
		rate:    limit,
		burst:   burst,
		limters: make(map[string]*rate.Limiter),
	}
}

func (r *RateLimiter) Allow(key string) bool {

	r.limitersLock.RLock()

	limit, ok := r.limters[key]

	r.limitersLock.RUnlock()

	if !ok {

		limit = rate.NewLimiter(r.rate, r.burst)

		r.limitersLock.Lock()
		r.limters[key] = limit
		r.limitersLock.Unlock()
	}

	return limit.Allow()
}
