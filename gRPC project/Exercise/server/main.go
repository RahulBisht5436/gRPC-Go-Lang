package main

import (
	"context"
	"fmt"
	"log"
	"net"

	sm "example.com/exercise/proto"
	"google.golang.org/grpc"
)

var ser = "0.0.0.0:3000"

type Server struct {
	sm.UnimplementedSumCalcServer
}

func (s *Server) SumCalculation(contx context.Context, in *sm.SumCalculationRequest) (*sm.SumCalculationResponse, error) {
	resultValue := in.GetNum1() + in.GetNum2()
	return &sm.SumCalculationResponse{
		Result: resultValue,
	}, nil
}

// net is a lib used to create TCP connections
func main() {
	lis, err := net.Listen("tcp", ser)
	if err != nil {
		log.Fatalf("Unable to Start the Server %v \n", err)
	}
	fmt.Printf("Server is started at the connection : %v \n", ser)
	s := grpc.NewServer()

	sm.RegisterSumCalcServer(s, &Server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to server : %v", err)
	}
}
