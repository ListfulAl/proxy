package proxy

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is a struct to hold configuration values.
type Config struct {
	RedisUrl         string
	RedisTTL         *time.Duration
	Port             string
	CacheKeyCapacity *int
	CacheTTL         *time.Duration
	ProxyClientLimit *int
	Mode             string
	AuthCode         string
	AuthTTL          *time.Duration
	DisableAuth      bool
	PermittedUsers   []string
	UserNameSpace    string
}

func (c Config) getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// NewConfig creates the struct Config by parsing environment
// vars or setting them to default values, if applicable
func NewConfig(dir *string) Config {
	c := Config{}
	c.RedisUrl = c.getEnv("REDIS_URL", "localhost:6379")
	log.Print("Importing Env Variables...")
	log.Print(fmt.Sprintf("REDIS_URL: %v", c.RedisUrl))
	port := c.getEnv("PORT", "8080")
	c.Port = fmt.Sprintf(":%v", port)
	log.Print(fmt.Sprintf("Port: %v", c.Port))
	ckp := c.getEnv("CACHE_KEY_CAPACITY", "")
	if ckp != "" {
		x, err := strconv.ParseInt(ckp, 10, 64)
		if err != nil {
			log.Fatal(err)
		} else {
			xc := int(x)
			c.CacheKeyCapacity = &xc
			log.Print(fmt.Sprintf("CACHE_KEY_CAPACITY: %v", xc))
		}

	}
	cttl := c.getEnv("CACHE_TTL", "")
	if cttl != "" {
		ct, err := time.ParseDuration(cttl + "s")
		if err != nil {
			log.Fatal(err)
		} else {
			c.CacheTTL = &ct
			log.Print(fmt.Sprintf("CACHE_TTL: %v", ct))
		}
	}
	rttl := c.getEnv("REDIS_TTL", "")
	if rttl != "" {
		rt, err := time.ParseDuration(rttl + "s")
		if err != nil {
			log.Fatal(err)
		} else {
			c.RedisTTL = &rt
		}
	}
	pcl := c.getEnv("PROXY_CLIENT_LIMIT", "")
	if pcl != "" {
		l, err := strconv.ParseInt(pcl, 10, 64)
		if err != nil {
			log.Fatal(err)
		} else {
			lc := int(l)
			c.ProxyClientLimit = &lc
		}
	}
	// interaction mode
	// 1 or "" - http
	// 2 is RESP
	c.Mode = c.getEnv("APP_MODE", "")

	// a code you must pass in order to create new name spaces
	// you should provide auth_code to your client
	c.AuthCode = c.getEnv("AUTH_CODE", "secret")
	attl := c.getEnv("AUTH_TTL", "3000")
	if attl != "" {
		rt, err := time.ParseDuration(attl + "s")
		if err != nil {
			log.Fatal(err)
		} else {
			c.AuthTTL = &rt
		}
	}

	if dir != nil {
		fileBytes, err := ioutil.ReadFile(fmt.Sprintf("%s/permit.txt", *dir))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		c.PermittedUsers = strings.Split(string(fileBytes), "\n")
		fmt.Println("Your permitted users")
		fmt.Println(c.PermittedUsers)
	}

	v := c.getEnv("DISABLE_AUTH", "true")
	bv, err := strconv.ParseBool(v)
	if err != nil {
		log.Fatal(err)
		fmt.Println(err)
		os.Exit(1)
	}
	c.DisableAuth = bv

	// where you want to nest your user data
	c.UserNameSpace = c.getEnv("USER_NAMESPACE", "userData")

	return c
}
