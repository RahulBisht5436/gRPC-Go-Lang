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


| #  | Generated Name                                 | Type               | Side   | Use Kab                                      |
| -- | ---------------------------------------------- | ------------------ | ------ | -------------------------------------------- |
| 1  | `<ServiceName>Client`                          | Interface          | Client | Client stub ka type                          |
| 2  | `<serviceName>Client` (lowercase first letter) | Struct (private)   | Client | Internal — ignore                            |
| 3  | `New<ServiceName>Client`                       | Function           | Client | Stub create karne ke liye                    |
| 4  | `<ServiceName>Server`                          | Interface          | Server | Server ko implement karna                    |
| 5  | `Unimplemented<ServiceName>Server`             | Struct             | Server | Apne struct mein embed karna                 |
| 6  | `Unsafe<ServiceName>Server`                    | Interface          | Server | Advanced — ignore                            |
| 7  | `Register<ServiceName>Server`                  | Function           | Server | `main.go` mein server register karne ke liye |
| 8  | `<ServiceName>_<MethodName>_FullMethodName`    | Const String       | Both   | Internal route — ignore                      |
| 9  | `_<ServiceName>_<MethodName>_Handler`          | Function (private) | Server | Internal dispatch — ignore                   |
| 10 | `<ServiceName>_ServiceDesc`                    | Variable (`var`)   | Server | Internal metadata — ignore                   |
