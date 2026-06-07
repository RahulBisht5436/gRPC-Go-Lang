package main

import (
	"context"
	"io"
	"log"
	"time"

	pb "example.com/bidirectional/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr = "localhost:8080"

func main() {

	names := []string{"Rahul bisht", "Sheetal Bisht", "kamal bisht", "Pareshwari Bishr"}
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
	stream, err := client.GreetEveryone(ctx)
	if err != nil {
		log.Printf("Unable to send the request , Reason : %v", err)
	}
	//Need to have more clear understanding of this part
	waitc := make(chan struct{})

	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("Recv error : %v ", err)
			}
			log.Printf("Response: %s", res.GetResult())
		}
	}()

	for _, name := range names {
		log.Printf("Sending: %s", name)
		if err := stream.Send(&pb.GreetRequest{FirstName: name}); err != nil {
			log.Fatalf("Send error: %v", err)
		}
		time.Sleep(300 * time.Millisecond) // optional: pace the sends
	}
	// 4. Tell server "no more requests" → triggers server's io.EOF
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("CloseSend error: %v", err)
	}
	// 5. Wait for the receive goroutine to finish (i.e., until server returns)
	<-waitc
	log.Println("Done")

}
