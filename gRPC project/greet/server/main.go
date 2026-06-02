package main

import (
	"log"
	"net"

	pb "example.com/greet/proto"
	"google.golang.org/grpc"
)

var addr = "0.0.0.0:50051"

// Server implements GreetServiceServer (RPC handlers live in greet.go).
// Embedding UnimplementedGreetServiceServer keeps us forward-compatible
// when new RPCs are added to greet.proto.
type Server struct {
	pb.UnimplementedGreetServiceServer
}

func main() {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	log.Printf("Listening on %s", addr)

	s := grpc.NewServer()
	pb.RegisterGreetServiceServer(s, &Server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	
}
