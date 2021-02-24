package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	redis "github.com/go-redis/redis/v8"
	assert "github.com/stretchr/testify/assert"
)

// Automated systems tests confirm that the end-to-end system functions as specified.
// These tests should treat the proxy as a black box to which an HTTP client connects and
// makes requests. The proxy itself should connect to a running Redis instance. The test
// should test the Redis proxy in its running state (i.e. by starting the artifact that would be
// started in production). It is also expected for the test to interact directly with the backing
// Redis instance in order to get it into a known good state (e.g. to set keys that would be
// read back through the proxy)

func TestProxyCachedGet(t *testing.T) {
	assert := assert.New(t)

	// some constants to start with
	duration, _ := time.ParseDuration("10s")

	config := NewConfig(nil)
	config.CacheTTL = &duration

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
	redisClient.Del(ctx, "roxi")
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(proxy.PayloadHandler)

	// Requirement: HTTP web service

	// test that calling get on empty cache will return nothing
	req, _ := http.NewRequest("GET", "/roxi", nil)
	handler.ServeHTTP(rr, req)
	assert.Equal(http.StatusNotFound, rr.Code)

	// set a value through the redis client and confirm you are able to pick it up
	// using the proxy
	// note this is using the redis client from the module and not proxy
	err = redisClient.Set(ctx, "roxi", "rocks", 0).Err()
	assert.NoError(err)

	rrAfterSet := httptest.NewRecorder()
	handler.ServeHTTP(rrAfterSet, req)
	assert.Equal(http.StatusOK, rrAfterSet.Code)
	assert.Equal(`{"roxi": "rocks"}`, rrAfterSet.Body.String())

	// Requirement: Cached GET
	// verify that map has value
	// this shows that the value is stored in the proxy cache
	// sleep to avoid test failure due to data race condition
	time.Sleep(2 * time.Second)
	assert.Equal("rocks", proxy.Data["roxi"].Value)

	// set the value again directly on redis to something else
	// this shows that the proxy is getting its value from the local cache
	// and not redis
	err = redisClient.Set(ctx, "roxi", "cute", 0).Err()
	assert.NoError(err)

	rrAfterSecondSet := httptest.NewRecorder()
	handler.ServeHTTP(rrAfterSecondSet, req)
	assert.Equal(http.StatusOK, rrAfterSecondSet.Code)
	assert.Equal(`{"roxi": "rocks"}`, rrAfterSecondSet.Body.String())

	// Requirement: Single backing instance
	// create another proxy and confirm that the value you get from it is the new value
	proxy2 := NewProxyCache(config)
	handler2 := http.HandlerFunc(proxy2.PayloadHandler)
	rr2 := httptest.NewRecorder()
	handler2.ServeHTTP(rr2, req)
	assert.Equal(http.StatusOK, rr2.Code)
	assert.Equal(`{"roxi": "cute"}`, rr2.Body.String())
	time.Sleep(2 * time.Second)
	assert.Equal("cute", proxy2.Data["roxi"].Value)
	// verify other is clearly same
	assert.Equal(proxy.Data["roxi"].Value, "rocks")
}

func TestGlobalExpiry(t *testing.T) {
	assert := assert.New(t)
	// Entries expire after some time that is globally configured in all proxy instances
	// this does not seem to apply to redis because it specifically mentions the proxy cache

	// mimic time passage

	// some constants to start with 1 sec ttl
	duration, _ := time.ParseDuration("1s")

	config := NewConfig(nil)
	config.CacheTTL = &duration

	// set up a few instances
	proxy1 := NewProxyCache(config)
	proxy2 := NewProxyCache(config)

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
	redisClient.Del(ctx, "tita")
	//rr := httptest.NewRecorder()
	handler1 := http.HandlerFunc(proxy1.PayloadHandler)
	handler2 := http.HandlerFunc(proxy2.PayloadHandler)

	// set value in redis
	err = redisClient.Set(ctx, "tita", "is cool", 0).Err()
	assert.NoError(err)

	// issue get request at the same time and verify the value is correct
	req, _ := http.NewRequest("GET", "/tita", nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		rr1 := httptest.NewRecorder()
		handler1.ServeHTTP(rr1, req)
		assert.Equal(`{"tita": "is cool"}`, rr1.Body.String())
	}()
	go func() {
		defer wg.Done()
		rr2 := httptest.NewRecorder()
		handler2.ServeHTTP(rr2, req)
		assert.Equal(`{"tita": "is cool"}`, rr2.Body.String())
	}()

	wg.Wait()
	// let time pass for tokens to expire
	time.Sleep(2 * time.Second)
	// set value in redis to new value
	err = redisClient.Set(ctx, "tita", "is fire", 0).Err()
	assert.NoError(err)

	// issue get request and verify the value is  new value
	//After an entry is expired, a GET request will
	//act as if the value associated with the key was never stored in the cache.
	wg.Add(2)
	go func() {
		defer wg.Done()
		rr1 := httptest.NewRecorder()
		handler1.ServeHTTP(rr1, req)
		assert.Equal(`{"tita": "is fire"}`, rr1.Body.String())
	}()
	go func() {
		defer wg.Done()
		rr2 := httptest.NewRecorder()
		handler2.ServeHTTP(rr2, req)
		assert.Equal(`{"tita": "is fire"}`, rr2.Body.String())
	}()

	wg.Wait()

}

func TestLRUEvictionFixedKeySize(t *testing.T) {
	assert := assert.New(t)

	// some constants to start with
	duration, _ := time.ParseDuration("1s")

	config := NewConfig(nil)
	config.CacheTTL = &duration
	only2 := 2
	config.CacheKeyCapacity = &only2

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

	// get rid of values for test
	redisClient.Del(ctx, "rocco")
	redisClient.Del(ctx, "heff")
	redisClient.Del(ctx, "tito")
	// set values in redis
	err = redisClient.Set(ctx, "rocco", "wow", 0).Err()
	assert.NoError(err)
	err = redisClient.Set(ctx, "heff", "zao", 0).Err()
	assert.NoError(err)
	err = redisClient.Set(ctx, "tito", "pow", 0).Err()
	assert.NoError(err)

	handler := http.HandlerFunc(proxy.PayloadHandler)

	// Requirement: LRU eviction
	// fetch data through the proxy

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rocco", nil)
	handler.ServeHTTP(rr, req)
	assert.Equal(rr.Code, http.StatusOK)
	assert.Equal(rr.Body.String(), `{"rocco": "wow"}`)

	req, _ = http.NewRequest("GET", "/heff", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(rr.Code, http.StatusOK)
	assert.Equal(rr.Body.String(), `{"heff": "zao"}`)
	time.Sleep(2 * time.Second)
	req, _ = http.NewRequest("GET", "/heff", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(rr.Code, http.StatusOK)
	assert.Equal(rr.Body.String(), `{"heff": "zao"}`)

	// this third call should displace data rocco
	req, _ = http.NewRequest("GET", "/tito", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(rr.Code, http.StatusOK)
	assert.Equal(rr.Body.String(), `{"tito": "pow"}`)

	// we should still have this data
	req, _ = http.NewRequest("GET", "/heff", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(rr.Code, http.StatusOK)
	assert.Equal(rr.Body.String(), `{"heff": "zao"}`)

	// and the third data should be empty
	// delete from redis backing to make results clear
	redisClient.Del(ctx, "rocco")
	rr = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/rocco", nil)
	handler.ServeHTTP(rr, req)
	assert.Equal(rr.Code, http.StatusNotFound)
	_, ok := proxy.Data["rocco"]
	assert.False(ok)

	_, ok2 := proxy.Data["tito"]
	assert.True(ok2)
	_, ok3 := proxy.Data["heff"]
	assert.True(ok3)

	//Requirement: Fixed key size
	assert.Equal(len(proxy.Data), 2)
}
