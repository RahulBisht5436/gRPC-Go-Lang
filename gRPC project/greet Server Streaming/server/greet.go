package main

import (
	"context"
	"fmt"
	"log"

	pb "example.com/greet/proto"
	"google.golang.org/grpc"
)

// Greet handles the unary GreetService.Greet RPC.
func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error) {
	log.Printf("Greet invoked with: %s", in.GetFirstName())
	return &pb.GreetResponse{
		Result: "Hello " + in.GetFirstName(),
	}, nil
}

func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream grpc.ServerStreamingServer[pb.GreetResponse]) error {
	fmt.Println("Stream Function initiated")

	for i := 0; i < 10; i++ {
		res := fmt.Sprintf("Changes for the User %s and time %d", in.FirstName, i)
		stream.Send(&pb.GreetResponse{
			Result: res,
		})
	}

	return nil
}
