module github.com/moderntv/cadre/_examples

go 1.23

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/gogo/protobuf v1.3.2
	github.com/moderntv/cadre v0.0.20
	github.com/rs/zerolog v1.29.0
	google.golang.org/grpc v1.53.0
)

replace github.com/moderntv/cadre => ../
