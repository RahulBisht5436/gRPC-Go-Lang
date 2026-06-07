package main

import (
	"io"
	"log"
	"net"

	pb "example.com/bidirectional/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedGreetServiceServer
}

var addr = "localhost:8080"

func (s *server) GreetEveryone(stream grpc.BidiStreamingServer[pb.GreetRequest, pb.GreetResponse]) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("recv error: %v", err)
			return err
		}

		respondMessage := "Hello, " + req.GetFirstName()
		if err := stream.Send(&pb.GreetResponse{Result: respondMessage}); err != nil {
			log.Printf("send error: %v", err)
			return err
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("unable to start listener: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterGreetServiceServer(s, &server{})

	log.Printf("Server started at %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("unable to start server: %v", err)
	}
}
