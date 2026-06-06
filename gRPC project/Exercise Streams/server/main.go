package main

import (
	"log"
	"net"

	pb "example.com/exercise/proto"
	"google.golang.org/grpc"
)

var addr string = "localhost:8080"

type server struct {
	pb.UnimplementedDivisorFinderServer
}

func (s *server) FindDivisors(req *pb.DivisorRequest, stream pb.DivisorFinder_FindDivisorsServer) error {
	num := req.GetNum()
	for i := uint32(1); i <= num; i++ {
		if num%i == 0 {
			if err := stream.Send(&pb.DivisorResponse{Result: i}); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to initiate the server: %v", err)
	}

	log.Printf("Server listening on %s", addr)

	s := grpc.NewServer()
	pb.RegisterDivisorFinderServer(s, &server{})

	if err := s.Serve(listen); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
