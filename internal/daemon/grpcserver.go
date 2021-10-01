package daemon

// import (
// 	"log"
// 	"net"

// 	"google.golang.org/grpc"
// 	"k8s.io/klog/v2"
// )

// const portNumber = "10000"

// func CreateServer() {
// 	lis, err := net.Listen("tcp", ":"+portNumber)
// 	if err != nil {
// 		log.Fatalf("failed to listen: %v", err)
// 	}

// 	grpcServer := grpc.NewServer()

// 	klog.Info("start gRPC server on %s port", portNumber)
// 	if err := grpcServer.Serve(lis); err != nil {
// 		log.Fatalf("failed to serve: %s", err)
// 	}
// }
