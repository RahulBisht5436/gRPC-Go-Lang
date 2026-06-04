package main

import (
	"context"
	"log"
	"time"

	sm "example.com/exercise/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var adddr = "localhost:3001"

func main() {
	conn, err := grpc.NewClient(
		adddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Unable to start the server")
	}

	defer conn.Close()

	client := sm.NewSumCalcClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.SumCalculation(ctx, &sm.SumCalculationRequest{
		Num1: 12,
		Num2: 24,
	})
	if err != nil {
		log.Fatalf("Not Able to call the Grpc procedure: %v", err)
	}
	log.Printf("This is the returned response : %v", res.GetResult())

}
