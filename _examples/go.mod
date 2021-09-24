module github.com/moderntv/cadre/_examples

go 1.16

require (
	github.com/gin-gonic/gin v1.6.2
	github.com/gogo/protobuf v1.3.2
	github.com/moderntv/cadre v0.0.20
	github.com/rs/zerolog v1.20.0
	google.golang.org/grpc v1.39.1
)

replace github.com/moderntv/cadre => ../
