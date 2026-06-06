package main

import (
	"context"
	"io"
	"log"
	"time"

	pb "example.com/greet/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr = "localhost:50051"

func main() {
	// insecure.NewCredentials() = no TLS (local dev only).
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to create gRPC client for %s: %v", addr, err)
	}
	defer conn.Close()

	client := pb.NewGreetServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.GreetManyTimes(ctx, &pb.GreetRequest{
		FirstName: "Rahul Bisht",
	})

	if err != nil {
		log.Fatalf("Greet RPC failed: %v", err)
	}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Streaming for the Request Failed")
		}
		log.Printf("Response: %s", res.GetResult())

	}
}
