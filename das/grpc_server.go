package das

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/RiemaLabs/nuport-offchain/proto"
)

type NubitDASRPCServer struct {
	nubitReader NubitReader
	nubitWriter NubitWriter
	proto.UnimplementedGeneralDAServer
}

func NewNubitDASRPCServer(cfg *DAConfig) *NubitDASRPCServer {
	da, err := NewNubitDA(cfg, nil)
	if nil != err {
		panic(err)
	}

	return &NubitDASRPCServer{
		nubitReader: da,
		nubitWriter: da,
	}
}

func (s *NubitDASRPCServer) Store(ctx context.Context, in *proto.StoreRequest) (*proto.StoreReply, error) {
	bpBytes, err := s.nubitWriter.Store(ctx, in.Data)
	if nil != err {
		return nil, err
	}

	return &proto.StoreReply{Receipt: bpBytes}, nil
}

func (s *NubitDASRPCServer) Read(ctx context.Context, in *proto.Receipt) (*proto.ReadReply, error) {
	var flag byte

	buf := bytes.NewReader(in.Data)
	if err := binary.Read(buf, binary.BigEndian, &flag); nil != err {
		return nil, err
	}

	if flag != NubitMessageHeaderFlag {
		return nil, fmt.Errorf("invalid flag: %v", flag)
	}

	bpBinary := make([]byte, len(in.Data)-1)
	if err := binary.Read(buf, binary.BigEndian, &bpBinary); err != nil {
		return nil, err
	}

	bp := &BlobPointer{}
	err := bp.UnmarshalBinary(bpBinary)
	if nil != err {
		return nil, err
	}
	result, err := s.nubitReader.Read(ctx, bp)
	if err != nil {
		return nil, err
	}

	return &proto.ReadReply{Result: result.Message}, nil
}
func (s *NubitDASRPCServer) GetProof(ctx context.Context, in *proto.Receipt) (*proto.ProofReply, error) {
	proof, err := s.nubitReader.GetProof(ctx, in.Data)
	if err != nil {
		return nil, err
	}
	return &proto.ProofReply{Proof: proof}, nil
}
func (s *NubitDASRPCServer) Verify(ctx context.Context, in *proto.VerifyRequest) (*proto.VerifyReply, error) {

	is, err := s.nubitReader.Verify(ctx, in.Proof)
	if nil != err {
		return nil, err
	}
	return &proto.VerifyReply{Is: is}, nil
}
