package main_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/RiemaLabs/nuport-offchain/config"
	"github.com/RiemaLabs/nuport-offchain/das"
	"github.com/ethereum/go-ethereum/log"
)

func main() {
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
	if nil != err {
		panic(err)
	}

	bpBytes, err := da.Store(context.Background(), []byte(`hello world`))
	if nil != err {
		panic(err)
	}

	var flag byte

	buf := bytes.NewReader(bpBytes)
	binary.Read(buf, binary.BigEndian, &flag)
	fmt.Println("Flag: ", flag)

	bpBinary := make([]byte, len(bpBytes)-1)
	binary.Read(buf, binary.BigEndian, &bpBinary)

	bp := &das.BlobPointer{}
	err = bp.UnmarshalBinary(bpBinary)
	if nil != err {
		panic(err)
	}

	fmt.Printf("BlobPointer: %#v \n", bp)

	readR, err := da.Read(context.Background(), bp)
	if nil != err {
		panic(err)
	}

	if !bytes.Equal([]byte(`hello world`), readR.Message) {
		fmt.Println("failed read:", readR.Message, "expected:", []byte(`hello world`))
	}
	fmt.Println("success!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!", "store data is equal to read data")
	// write file

	os.WriteFile(fmt.Sprintf("%d.txt", bp.BlockHeight), bpBytes, 0644)

}
