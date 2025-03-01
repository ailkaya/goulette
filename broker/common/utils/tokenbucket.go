package utils

import (
	"context"
	"time"
)

// TokenBucket 令牌桶算法
func TokenBucket(ctx context.Context, buckets chan struct{}, interval time.Duration, updateInterval chan time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for _ = range ticker.C {
		select {
		case <-ctx.Done():
			return
		case newInterval := <-updateInterval:
			interval = newInterval
		case buckets <- struct{}{}:
		default:
		}
		ticker.Reset(interval)
	}
}
