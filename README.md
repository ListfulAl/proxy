# proxy
[![Go Report Card](https://goreportcard.com/badge/github.com/ListfulAl/proxy)](https://goreportcard.com/report/github.com/ListfulAl/proxy)

Proxy is a package to build a web server with a local cache and an external cache, like redis.


## Installation

Start up service and run tests:

```bash
make test
```

Build proxy and redis contatiners and run, without running tests:

```bash
docker-compose up --build
```

If you have the redis server running in the background, and you do not have the proxy running in the background:

```bash
make build-proxy
./bin/proxy
```

Test stashing a value and key:

```bash
curl -X PUT -d "cool" localhost:8080/roxi
```

Note that the item after the route is the key you want to assign.

To get the value, simply use the key as the route.

```bash
curl localhost:8080/roxi
```

If you have redis-cli installed, you can use it to view your keys.

```bash
➜ redis-cli
127.0.0.1:6379> keys *
1) "roxi"
```

## High-level architecture overview

This module has two main components:

- proxy: A go app that has its own in-memory cache. Interacts with external cache for synchronization among other proxy instances.

- external cache: At the moment this is Redis. A cache that is run in its own docker contatiner and is also
    an in-memory data store that is run seperately from the proxy.


The build is managed with Makefile and docker-compose.yml. The app is written in go.
