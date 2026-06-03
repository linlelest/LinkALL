// 内存版限流器（基于滑动窗口令牌桶）
// 支持按 key 组合限流（IP、IP+账号、IP+WS 节点）
package security

import (
	"sync"
	"time"
)

type Bucket struct {
	Capacity   int
	RefillRate float64 // tokens per second
	Tokens     float64
	LastRefill time.Time
}

type Rule struct {
	Key       string
	Capacity  int
	RefillSec int // 多少秒填满一次
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*Bucket
	rules   map[string]Rule
	stop    chan struct{}
}

func NewLimiter() *Limiter {
	l := &Limiter{
		buckets: map[string]*Bucket{},
		rules:   map[string]Rule{},
		stop:    make(chan struct{}),
	}
	go l.gc()
	return l
}

func (l *Limiter) SetRule(key string, capacity, refillSec int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rules[key] = Rule{Key: key, Capacity: capacity, RefillSec: refillSec}
}

func (l *Limiter) Allow(scope, identity string) (bool, Rule) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.rules[scope]
	if !ok {
		return true, Rule{}
	}
	k := scope + "|" + identity
	b, ok := l.buckets[k]
	if !ok {
		b = &Bucket{
			Capacity:   r.Capacity,
			RefillRate: float64(r.Capacity) / float64(r.RefillSec),
			Tokens:     float64(r.Capacity),
			LastRefill: time.Now(),
		}
		l.buckets[k] = b
	}
	now := time.Now()
	elapsed := now.Sub(b.LastRefill).Seconds()
	b.Tokens += elapsed * b.RefillRate
	if b.Tokens > float64(b.Capacity) {
		b.Tokens = float64(b.Capacity)
	}
	b.LastRefill = now
	if b.Tokens >= 1 {
		b.Tokens -= 1
		return true, r
	}
	return false, r
}

func (l *Limiter) Reset(scope, identity string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, scope+"|"+identity)
}

func (l *Limiter) gc() {
	t := time.NewTicker(2 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			l.mu.Lock()
			now := time.Now()
			for k, b := range l.buckets {
				if now.Sub(b.LastRefill) > 10*time.Minute {
					delete(l.buckets, k)
				}
			}
			l.mu.Unlock()
		}
	}
}

func (l *Limiter) Stop() { close(l.stop) }
