package examples

import (
	"context"
	"fmt"

	greeter_pb "github.com/moderntv/cadre/examples/proto/greeter"
)

// GreeterService is a trivial implementation of the example.GreeterService gRPC service.
type GreeterService struct {
	greeter_pb.UnimplementedGreeterServiceServer
}

func (gs *GreeterService) SayHi(
	ctx context.Context,
	in *greeter_pb.GreetingRequest,
) (response *greeter_pb.GreetingResponse, err error) {
	response = &greeter_pb.GreetingResponse{
		Greeting: fmt.Sprintf("Hi, %s!", in.GetName()),
	}

	return
}
