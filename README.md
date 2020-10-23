# Cadre
Cadre is a strongly opinionated library intended to removed boilerplate code from a modern Go application supporting gRPC and HTTP. It has been build for internal projects needs at [ModernTV](https://www.moderntv.eu).

Cadre makes it easy to create and application with gRPC and/or HTTP interface. It provides prometheus metrics and application status endpoints, debugging tools, logging and various gRPC utils.

Cadre tries to be flexible but enforces several libraries:
* logging - [zerolog](https://github.com/rs/zerolog)
* http server - [gin](https://github.com/gin-gonic/gin)

See `_examples` folder for usage details.

## Disclaimer
Cadre is not production ready. It is under heavy development and its API can be changed at any time.

## Why Cadre?
[Cadre](https://www.wordnik.com/words/cadre)
