// Command cli is a minimal gRPC client for the greeter service exposed by the other examples.
//
//	go run ./cli -name Alice
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	greeterpb "github.com/moderntv/cadre/examples/proto/greeter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	name := flag.String("name", "Bob", "name of the person to greet")
	addr := flag.String("addr", "localhost:9000", "address of the greeter gRPC server")

	flag.Parse()

	cc, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer cc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	greeterClient := greeterpb.NewGreeterServiceClient(cc)

	res, err := greeterClient.SayHi(ctx, &greeterpb.GreetingRequest{Name: *name})
	if err != nil {
		panic(err)
	}

	fmt.Println(res.GetGreeting())
}
