package main

import (
	"fmt"

	"net"
	"os"

	"github.com/RiemaLabs/nuport-offchain/config"
	"github.com/RiemaLabs/nuport-offchain/das"
	pb "github.com/RiemaLabs/nuport-offchain/proto"
	"github.com/ethereum/go-ethereum/log"
	"google.golang.org/grpc"
)

func main() {
	if err := startup(); err != nil {
		log.Error("Error running NubitDAServer", "err", err)
	}
}

func startup() error {
	cfg := config.NewConfig()

	glogger := log.NewGlogHandler(log.JSONHandlerWithLevel(os.Stderr, log.LvlDebug))
	glogger.Verbosity(log.LvlDebug)
	log.SetDefault(log.NewLogger(glogger))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.RPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	ns := das.NewNubitDASRPCServer(&cfg.NubitDa)
	pb.RegisterGeneralDAServer(s, ns)
	log.Info("server listening at", "address", lis.Addr())
	if err := s.Serve(lis); err != nil {
		return err
	}
	return nil
}
