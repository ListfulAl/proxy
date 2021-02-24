package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	redis "github.com/go-redis/redis/v8"
	assert "github.com/stretchr/testify/assert"
)

func TestSequentialProcessing(t *testing.T) {
	// Use middleware and pass 1 as an argument
	// this is the same as setting the config.ProxyClientLimit to 1
	// also known as "PROXY_CLIENT_LIMIT" env var

	// normally I would just test the middleware function with mocks
	// but since the requirement is to prove the proxy can perform and still use
	// redis backing cache I do not use mocks
	assert := assert.New(t)

	config := NewConfig(nil)
	proxy := NewProxyCache(config)

	var ctx = context.Background()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisUrl,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	// redis client that should be running
	_, err := redisClient.Ping(ctx).Result()
	assert.NoError(err)

	// get rid of value for test
	redisClient.Del(ctx, "bing")
	redisClient.Del(ctx, "bong")
	err = redisClient.Set(ctx, "bing", "charlie", 0).Err()
	assert.NoError(err)
	err = redisClient.Set(ctx, "bong", "is cool", 0).Err()
	assert.NoError(err)

	handler := http.HandlerFunc(LimitNumClients(proxy.PayloadHandler, 1))

	req1, _ := http.NewRequest("GET", "/bing", nil)
	req2, _ := http.NewRequest("GET", "/bong", nil)
	req3, _ := http.NewRequest("GET", "/zeep", nil)
	req4, _ := http.NewRequest("GET", "/zoot", nil)
	var wg sync.WaitGroup
	wg.Add(6)
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req1)
		assert.Equal(`{"bing": "charlie"}`, rr.Body.String())
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req2)
		assert.Equal(`{"bong": "is cool"}`, rr.Body.String())
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req1)
		assert.Equal(`{"bing": "charlie"}`, rr.Body.String())
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req2)
		assert.Equal(`{"bong": "is cool"}`, rr.Body.String())
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req3)
		assert.Equal(http.StatusNotFound, rr.Code)
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req4)
		assert.Equal(http.StatusNotFound, rr.Code)

	}()
	wg.Wait()
}

func TestNonSequentialProcessing(t *testing.T) {
	// do the same thing but set the value to something else like 10?
	assert := assert.New(t)

	config := NewConfig(nil)
	proxy := NewProxyCache(config)

	var ctx = context.Background()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisUrl,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	// redis client that should be running
	_, err := redisClient.Ping(ctx).Result()
	assert.NoError(err)

	// get rid of value for test
	redisClient.Del(ctx, "bing")
	redisClient.Del(ctx, "bong")
	err = redisClient.Set(ctx, "bing", "charlie", 0).Err()
	assert.NoError(err)
	err = redisClient.Set(ctx, "bong", "is cool", 0).Err()
	assert.NoError(err)

	// this is how we can permit concurrent processing that is not sequential
	handler := http.HandlerFunc(LimitNumClients(proxy.PayloadHandler, 10))
	req1, _ := http.NewRequest("GET", "/bing", nil)
	req2, _ := http.NewRequest("GET", "/bong", nil)
	req3, _ := http.NewRequest("GET", "/zeep", nil)
	req4, _ := http.NewRequest("GET", "/zoot", nil)
	var wg sync.WaitGroup
	wg.Add(6)
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req1)
		assert.Equal(rr.Body.String(), `{"bing": "charlie"}`)
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req2)
		assert.Equal(rr.Body.String(), `{"bong": "is cool"}`)
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req1)
		assert.Equal(rr.Body.String(), `{"bing": "charlie"}`)
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req2)
		assert.Equal(rr.Body.String(), `{"bong": "is cool"}`)
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req3)
		assert.Equal(rr.Code, http.StatusNotFound)
	}()
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req4)
		assert.Equal(rr.Code, http.StatusNotFound)

	}()
	wg.Wait()
}
