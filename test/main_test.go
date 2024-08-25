package main_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/RiemaLabs/nuport-offchain/config"
	"github.com/RiemaLabs/nuport-offchain/das"
	"github.com/RiemaLabs/nuport-offchain/proto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(t *testing.T) {
	cfg := config.NewConfig()
	glogger := log.NewGlogHandler(log.JSONHandlerWithLevel(os.Stderr, log.LvlDebug))
	glogger.Verbosity(log.LvlDebug)
	log.SetDefault(log.NewLogger(glogger))

	da, err := das.NewNubitDA(&das.DAConfig{
		GasPrice:      0.01,
		GasMultiplier: 1.01,
		Rpc:           cfg.NubitDa.Rpc,
		NamespaceId:   cfg.NubitDa.NamespaceId,
		AuthToken:     cfg.NubitDa.AuthToken,
		NoopWriter:    false,
		ValidatorConfig: &das.ValidatorConfig{
			TendermintRPC: cfg.NubitDa.ValidatorConfig.TendermintRPC,
			EthClient:     cfg.NubitDa.ValidatorConfig.EthClient,
			NuportAddr:    cfg.NubitDa.ValidatorConfig.NuportAddr,
			VerifyAddr:    cfg.NubitDa.ValidatorConfig.VerifyAddr,
		},
	}, nil)
	require.NoError(t, err)

	bpBytes, err := da.Store(context.Background(), []byte("hello world"))
	require.NoError(t, err)

	var flag byte

	buf := bytes.NewReader(bpBytes)
	binary.Read(buf, binary.BigEndian, &flag)
	fmt.Println("Flag: ", flag)

	bpBinary := make([]byte, len(bpBytes)-1)
	binary.Read(buf, binary.BigEndian, &bpBinary)

	bp := &das.BlobPointer{}
	err = bp.UnmarshalBinary(bpBinary)
	require.NoError(t, err)

	fmt.Printf("BlobPointer: %#v \n", bp)
	readR, err := da.Read(context.Background(), bp)
	require.NoError(t, err)

	t.Logf("ReadResult: %#v", readR)

	require.Equal(t, []byte(`hello world`), readR.Message)
}

func TestReadResult(t *testing.T) {
	cfg := config.NewConfig()
	glogger := log.NewGlogHandler(log.JSONHandlerWithLevel(os.Stderr, log.LvlDebug))
	glogger.Verbosity(log.LvlDebug)
	log.SetDefault(log.NewLogger(glogger))

	da, err := das.NewNubitDA(&das.DAConfig{
		GasPrice:      0.01,
		GasMultiplier: 1.01,
		Rpc:           cfg.NubitDa.Rpc,
		NamespaceId:   cfg.NubitDa.NamespaceId,
		AuthToken:     cfg.NubitDa.AuthToken,
		NoopWriter:    false,
		ValidatorConfig: &das.ValidatorConfig{
			TendermintRPC: cfg.NubitDa.ValidatorConfig.TendermintRPC,
			EthClient:     cfg.NubitDa.ValidatorConfig.EthClient,
			NuportAddr:    cfg.NubitDa.ValidatorConfig.NuportAddr,
			VerifyAddr:    cfg.NubitDa.ValidatorConfig.VerifyAddr,
		},
	}, nil)
	require.NoError(t, err)

	height := uint64(17846)

	bpBytes, err := os.ReadFile(fmt.Sprintf("%d.txt", height))
	require.NoError(t, err)

	pbytes, err := da.GetProof(context.Background(), bpBytes)
	if nil != err {
		panic(err)
	}

	is, err := da.Verify(context.Background(), pbytes)
	if nil != err {
		fmt.Println("failed to verify proof", "error", err)
	}

	if !is {
		fmt.Println("failed to verify proof")
	}
}

func TestRpc_Store_Read(t *testing.T) {
	conn, err := grpc.NewClient(":9876", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	defer conn.Close()
	c := proto.NewGeneralDAClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	bpBytes, err := c.Store(ctx, &proto.StoreRequest{Data: []byte("hello world")})
	require.NoError(t, err)

	var flag byte
	buf := bytes.NewReader(bpBytes.Receipt)
	binary.Read(buf, binary.BigEndian, &flag)
	require.Equal(t, das.NubitMessageHeaderFlag, flag)

	bpBinary := make([]byte, len(bpBytes.Receipt)-1)
	binary.Read(buf, binary.BigEndian, &bpBinary)

	bp := &das.BlobPointer{}
	err = bp.UnmarshalBinary(bpBinary)
	require.NoError(t, err)

	fmt.Printf("BlobPointer: %#v \n", bp)
	readR, err := c.Read(ctx, &proto.Receipt{Data: bpBytes.Receipt})
	require.NoError(t, err)

	fmt.Printf("ReadResult: %v\n", readR.Result)

	require.Equal(t, []byte(`hello world`), readR.Result)
}

func TestRpc_GetProof_Verify(t *testing.T) {
	conn, err := grpc.NewClient(":9876", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	defer conn.Close()
	c := proto.NewGeneralDAClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	height := uint64(17846)

	bpBytes, err := os.ReadFile(fmt.Sprintf("%d.txt", height))
	require.NoError(t, err)

	p, err := c.GetProof(ctx, &proto.Receipt{Data: bpBytes})
	require.NoError(t, err)

	v, err := c.Verify(ctx, &proto.VerifyRequest{Proof: p.Proof})
	if nil != err {
		fmt.Println("failed to verify proof", "error", err)
	}

	if !v.Is {
		fmt.Println("failed to verify proof")
	}
}
