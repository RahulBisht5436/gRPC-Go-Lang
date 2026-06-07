package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "example.com/clientStream/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr string = "localhost:8080"

func main() {
	connc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Not able to stablish the connection %v", err)
	}
	defer connc.Close()

	client := pb.NewUserServiceClient(connc)
	coxt, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	stream, err := client.SendUser(coxt)
	if err != nil {
		log.Fatalf("streaming didn't started %v", err)
	}

	names := []string{"Rahul Bisht", "Sheetal Bisht", "Kamal Bisht", "Pareshwari Bisht"}
	for _, name := range names {
		log.Printf("Sending the name : %v", name)
		err := stream.Send(&pb.UsersRequest{
			Name: name,
		})
		if err != nil {
			log.Printf("Not able to send the Request : %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Failed Request : %v", err)
	}

	fmt.Println("Successful Client Stream Done : %v", resp.GetResult())

}
