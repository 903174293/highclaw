package config

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Cache 带 TTL 的配置缓存，支持运行时动态感知配置文件变更。
// 读取时若缓存过期则自动从磁盘重新加载，避免每次请求都读文件。
type Cache struct {
	mu       sync.RWMutex
	config   *Config
	hash     string // 当前配置内容的 SHA-256 hash，用于乐观锁
	loadedAt time.Time
	ttl      time.Duration
}

// NewCache 创建配置缓存，initialCfg 为启动时已加载的配置
func NewCache(initialCfg *Config, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 500 * time.Millisecond
	}
	h := computeConfigHash(initialCfg)
	return &Cache{
		config:   initialCfg,
		hash:     h,
		loadedAt: time.Now(),
		ttl:      ttl,
	}
}

// Get 返回当前配置；缓存过期时自动从磁盘重新加载
func (c *Cache) Get() *Config {
	c.mu.RLock()
	if time.Since(c.loadedAt) < c.ttl {
		cfg := c.config
		c.mu.RUnlock()
		return cfg
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// double-check：可能另一个 goroutine 已经刷新
	if time.Since(c.loadedAt) < c.ttl {
		return c.config
	}

	cfg, err := Load()
	if err != nil {
		// 加载失败则继续使用旧配置，延长 TTL 避免频繁重试
		c.loadedAt = time.Now()
		return c.config
	}
	c.config = cfg
	c.hash = computeConfigHash(cfg)
	c.loadedAt = time.Now()
	return c.config
}

// Hash 返回当前配置的 SHA-256 hash，用于乐观并发控制
func (c *Cache) Hash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hash
}

// Invalidate 使缓存立即过期，下次 Get 将重新从磁盘加载
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadedAt = time.Time{}
}

// Set 直接替换缓存中的配置（写入文件后调用，避免等 TTL 过期）
func (c *Cache) Set(cfg *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = cfg
	c.hash = computeConfigHash(cfg)
	c.loadedAt = time.Now()
}

// computeConfigHash 计算配置的 SHA-256 摘要
func computeConfigHash(cfg *Config) string {
	data, err := marshalConfigYAML(cfg)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
