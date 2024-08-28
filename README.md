[![Go Report Card](https://goreportcard.com/badge/github.com/moderntv/cadre)](https://goreportcard.com/report/github.com/moderntv/cadre)
![Go Version](https://img.shields.io/github/go-mod/go-version/moderntv/cadre)
![Lint Workflow Status](https://github.com/moderntv/cadre/actions/workflows/golangci-lint.yml/badge.svg?branch=master)

# Cadre

Cadre is a strongly opinionated library intended to removed boilerplate code from a modern Go application supporting gRPC and HTTP. 
It has been build for internal projects needs at [ModernTV](https://www.moderntv.eu).

Cadre makes it easy to create and application with gRPC and/or HTTP interface. 
It provides prometheus metrics and application status endpoints, debugging tools, logging and various gRPC utils.

Cadre tries to be flexible but enforces several libraries:
* logging - [zerolog](https://github.com/rs/zerolog)
* http server - [gin](https://github.com/gin-gonic/gin)

See `_examples` folder for usage details.

## Disclaimer

Cadre is under heavy development and its API can be changed at any time.

## Default options

Cadre by default handles `SIGINT` and `SIGTERM` signals when using `WithFinisher` option. The path for status component runs on `/status` endpoint and for metrics it is exposed on `/metrics` endpoint.
When `WithHTTPListeningAddress` is provided, the endpoints are running on given address.

### HTTP

- By default the metrics are not aggregated under `*` character when using routes ended with `*`. Use `WithMetricsAggregation` to aggregate metrics same as on handler side.
- Logging using `zerolog` is by default on. Use `WithoutLoggingMiddleware` to prevent this behaviour.
- Metrics are gathered by default which could be exposed using `/metrics` endpoint. Use `WithoutMetricsMiddleware` to prevent this behaviour.

### gRPC

- By default the recovery middleware is on. Use `WithoutRecoveryMiddleware` option to prevent this behaviour.
- By default reflection is on. Use `WithoutReflection` to prevent reflection.
- By default the logging middleware is on. It is possible to prevent the logging using `WithoutLogging` option. 
- Health service is by default on. It could be prevented using `WithoutHealthService`. For health service the `grpc_health_v1` library is used.
- When channelz is enabled, it is running on port `8192`. It is exposed as `GET` handler on `/channelz/*path`. For channelz the `go-grpc-channelz` is used.
	
## Config

Cadre enables to bind configuration of http/gRPC handlers. By default the `viper` is used as config parser.

It currently supports 2 configuration formats:

1. YAML
2. JSON

As the YAML or JSON file is provided with valid configuration we often change it. Cadre uses file `watcher` using `fsnotify` to dynamically change running configuration of your program.

## HTTP responses

As the cadre is opinionated library it also provides ready made HTTP responses. There is special response structure where we provide function name with  error. It constructs adequate `status code` as response. 

## Sharding load balancer

Cadre also supports usage of sharded load balancer. It uses `hashring` algorithm to provide sharded load balancer. 

## Metrics

As the prometheus is used to register the metrics and propagate it on `/metrics` endpoint by default, the exposure of new metrics is done by cadre `collectors`. It has wrappers over `prometheus` library to `Register` new metrics by your own choice and expose it on `/metrics` endpoint or name of your choice.

## Registry

GRPC uses resolver to connect with other gRPC services. For such purpose there exists generic registry interface where we can provide the hosts to be connected with using particular registry (e.g `etcd`). Typicall usage is `file` registry, where we provide a file with configuration of hosts to be connected with using `registry:///yourservice`.

## Why Cadre?

[Cadre](https://www.wordnik.com/words/cadre)
