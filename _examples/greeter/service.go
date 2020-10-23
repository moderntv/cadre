package greeter

import (
	"context"
	"fmt"

	greeter_pb "github.com/moderntv/cadre/_example/proto/greeter"
)

type GreeterService struct {
	greeter_pb.UnimplementedGreeterServiceServer
}

func (gs *GreeterService) SayHi(ctx context.Context, in *greeter_pb.GreetingRequest) (response *greeter_pb.GreetingResponse, err error) {
	response = &greeter_pb.GreetingResponse{
		Greeting: fmt.Sprintf("Hi, %s!", in.Name),
	}

	return
}
