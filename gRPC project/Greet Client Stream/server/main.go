package main

import (
	"io"
	"log"
	"net"
	"strings"

	pb "example.com/clientStream/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedUserServiceServer
}

var addr string = "localhost:8080"

func (s *server) SendUser(stream grpc.ClientStreamingServer[pb.UsersRequest, pb.UserResponse]) error {
	var names []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			result := "hello, " + strings.Join(names, ", ")
			return stream.SendAndClose(&pb.UserResponse{
				Result: result,
			})
		}
		if err != nil {
			log.Printf("recv error: %v", err)
			return err // ← return, don't Fatalf
		}

		log.Printf("Received name: %s", req.GetName())
		names = append(names, req.GetName())
	}
}

func main() {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Not able to initiate the server: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &server{})

	log.Printf("Server listening on %s", addr) // ← print BEFORE Serve
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
