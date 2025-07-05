package handler

import (
	"context"
	"fmt"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth"
)

func (h *Handler) Hello(_ context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	greeting := fmt.Sprintf("Hello, %s!", req.GetName())

	return &pb.HelloResponse{
		Greeting: greeting,
	}, nil
}
