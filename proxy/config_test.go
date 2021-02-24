package proxy

import (
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
)

func TestConfiguration(t *testing.T) {
	assert := assert.New(t)
	// verify that config are defined by the environment, if it is defined
	// in the environment
	os.Setenv("REDIS_URL", "1")
	os.Setenv("REDIS_TTL", "3")
	os.Setenv("PORT", "3")
	os.Setenv("CACHE_KEY_CAPACITY", "4")
	os.Setenv("CACHE_TTL", "5")
	os.Setenv("PROXY_CLIENT_LIMIT", "6")

	e1, _ := time.ParseDuration("3s")
	e2, _ := time.ParseDuration("5s")
	config := NewConfig(nil)
	assert.Equal("1", config.RedisUrl)
	assert.Equal(e1, *config.RedisTTL)
	assert.Equal(":3", config.Port)
	assert.Equal(4, *config.CacheKeyCapacity)
	assert.Equal(e2, *config.CacheTTL)
	assert.Equal(6, *config.ProxyClientLimit)

	os.Unsetenv("REDIS_URL")
	os.Unsetenv("REDIS_TTL")
	os.Unsetenv("PORT")
	os.Unsetenv("CACHE_KEY_CAPACITY")
	os.Unsetenv("CACHE_TTL")
	os.Unsetenv("PROXY_CLIENT_LIMIT")
}
