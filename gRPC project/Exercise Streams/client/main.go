package main

import (
	"context"
	"io"
	"log"
	"time"

	pb "example.com/exercise/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var clientAddr = "localhost:8080"

func main() {
	connc, err := grpc.NewClient(clientAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Unable to establish the connection %v", err)
	}
	defer connc.Close()

	client := pb.NewDivisorFinderClient(connc)

	contx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, streamError := client.FindDivisors(contx, &pb.DivisorRequest{
		Num: 122,
	})
	if streamError != nil {
		log.Fatalf("FindDivisors RPC failed: %v", streamError)
	}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Streaming failed: %v", err)
		}
		log.Printf("Response: %d", res.GetResult())
	}

}
