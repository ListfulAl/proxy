package proxy

import (
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"sync"
	"time"
)

type ValueStore struct {
	LastRead   time.Time
	Value      string
	ExpiryTime time.Time
}

// ProxyCache is a cache used by the proxy that is safe to use concurrently
type ProxyCache struct {
	Data map[string]ValueStore
	Mux  sync.Mutex

	// MaxKeys optionally limits the total number of keys stored in
	// the Data map at any time
	//
	// Zero means no limit
	MaxKeys int

	// KeyTimeout is the maximum amount of time the key
	// will remain in data until it is expired in Seconds
	// Zero means no limit
	KeyTimeout time.Duration

	// Cache is a cache used by the proxy that is not in-memory storage
	cache Cache
}

// Put ...
func (c *ProxyCache) Put(key string, value string) {

	c.Mux.Lock()
	defer c.Mux.Unlock()

	// only purge LLU if max key limit set
	if c.MaxKeys != 0 && len(c.Data) == c.MaxKeys {
		lastKey := ""
		lastRead := time.Now()
		for k := range c.Data {
			v, ok := c.Data[k]
			if ok && v.LastRead.Before(lastRead) {
				lastKey = k
				lastRead = v.LastRead
			}
		}

		// remove the key that was accessed a longest time
		delete(c.Data, lastKey)
	}

	c.Data[key] = ValueStore{
		Value:      value,
		LastRead:   time.Now(),
		ExpiryTime: time.Now().Add(time.Second * c.KeyTimeout),
	}
}

// Get ...
func (c *ProxyCache) Get(key string) *string {

	c.Mux.Lock()
	defer c.Mux.Unlock()

	value, ok := c.Data[key]

	if ok {
		value.LastRead = time.Now()
		c.Data[key] = value
		return &value.Value
	}

	return nil
}

// ExpireKeys ...
func (c *ProxyCache) ExpireKeys() {

	// Iterate through all keys in the map to check if any have expired
	// call from a go routine so that this is done async
	go func() {
		for true {

			c.Mux.Lock()

			keysToExpire := []string{}
			for k := range c.Data {
				v, ok := c.Data[k]
				if ok && v.ExpiryTime.Before(time.Now()) {
					keysToExpire = append(keysToExpire, k)
				}
			}

			for _, k := range keysToExpire {
				delete(c.Data, k)
			}

			c.Mux.Unlock()

			// sleep for the duration of the timeout
			time.Sleep(time.Second * c.KeyTimeout)
		}
	}()
}

// PayloadHandler ...
func (c *ProxyCache) PayloadHandler(w http.ResponseWriter, r *http.Request) {
	key := path.Base(r.URL.String())

	if key == "/" {
		w.Write([]byte("BAD"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		value := c.Get(key)
		if value != nil {
			w.Write([]byte(*value))
			return
		}

		// try to get key value from external cache
		cv, err := c.cache.Get(key)
		if err != nil {
			log.Fatal(err)
			w.Write([]byte("BAD"))
			return
		} else if cv == nil {
			w.Write([]byte("BAD"))
			return
		}

		// store the value in the proxy cache
		go c.Put(key, *cv)
		w.Write(([]byte(*cv)))

	case http.MethodPut:
		value, err := ioutil.ReadAll(r.Body)

		if err != nil {
			log.Fatal(err)
			w.Write([]byte("BAD"))
			return
		}

		go c.Put(key, string(value))

		err = c.cache.Put(key, string(value))
		if err != nil {
			log.Fatal(err)
			w.Write([]byte("BAD"))
			return
		}

		w.Write([]byte("OK"))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// NewProxyCache constructs a new ProxyCache complete with an external cache
func NewProxyCache(config Config) *ProxyCache {
	pc := ProxyCache{
		Data: make(map[string]ValueStore),
	}

	if config.CacheKeyCapacity != nil {
		pc.MaxKeys = *config.CacheKeyCapacity
	}

	if config.CacheTTL != nil {
		pc.KeyTimeout = *config.CacheTTL
		// call method so that it can check what keys can expire
		pc.ExpireKeys()
	}
	// set up external cache
	pc.cache = NewRedisClient(config.CacheTTL, config.RedisUrl)

	return &pc
}
