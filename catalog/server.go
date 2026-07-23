//go:generate protoc --go_out=./pb --go-grpc_out=./pb catalog.proto
package catalog

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/trunglq04/go-grpc-graphql/catalog/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// This server GET REQUESTS from other services (Inbound)
// Get requests and process it in service

type grpcServer struct {
	// Impl it help not getting interrupted when update the codebase
	pb.UnimplementedCatalogServiceServer         // Forward compat
	service                              Service // Business logic
}

func ListenGRPC(s Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))	// Open port
	if err != nil {
		return err // err if cannot use the port
	}
	serv := grpc.NewServer() // Create gRPC server container without Interceptor (gRPC middleware)
	pb.RegisterCatalogServiceServer(serv, &grpcServer{service: s}) // Map routes + handlers
	reflection.Register(serv) // Turn on reflection (debug), which allow client to discover service schema at runtime, should be turn off on production due to public the schema
	return serv.Serve(lis)	// Run server loop
}

func (s *grpcServer) PostProduct(ctx context.Context, r *pb.PostProductRequest) (*pb.PostProductResponse, error) {
	p, err := s.service.PostProduct(ctx, r.Name, r.Description, r.Price)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &pb.PostProductResponse{Product: &pb.Product{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
	}}, nil
}

func (s *grpcServer) GetProduct(ctx context.Context, r *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	p, err := s.service.GetProduct(ctx, r.Id)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &pb.GetProductResponse{
		Product: &pb.Product{
			Id:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
		},
	}, nil
}

func (s *grpcServer) GetProducts(ctx context.Context, r *pb.GetProductsRequest) (*pb.GetProductsResponse, error) {
	var res []Product
	var err error
	if r.Query != "" {
		res, err = s.service.SearchProducts(ctx, r.Query, r.Skip, r.Take)
	} else if len(r.Ids) != 0 {
		res, err = s.service.GetProductsByIDs(ctx, r.Ids)
	} else {
		res, err = s.service.GetProducts(ctx, r.Skip, r.Take)
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}

	products := []*pb.Product{}
	for _, p := range res {
		products = append(
			products,
			&pb.Product{
				Id:          p.ID,
				Name:        p.Name,
				Description: p.Description,
				Price:       p.Price,
			},
		)
	}
	return &pb.GetProductsResponse{Products: products}, nil
}
