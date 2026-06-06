package main

import (
	"context"
	"log"

	pb "example.com/greet/proto"
)

// Greet handles the unary GreetService.Greet RPC.
func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error) {
	log.Printf("Greet invoked with: %s", in.GetFirstName())
	return &pb.GreetResponse{
		Result: "Hello " + in.GetFirstName(),
	}, nil
}
