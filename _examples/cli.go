package main

import (
	"context"
	"flag"
	"fmt"

	greeterpb "github.com/moderntv/cadre/_example/proto/greeter"
	"google.golang.org/grpc"
)

func main() {
	name := flag.String("name", "Bob", "Name of the person to greet")
	flag.Parse()

	cc, err := grpc.Dial("localhost:9000", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	greeterClient := greeterpb.NewGreeterServiceClient(cc)
	res, err := greeterClient.SayHi(context.Background(), &greeterpb.GreetingRequest{Name: *name})
	if err != nil {
		panic(err)
	}

	fmt.Println(res.Greeting)
}
