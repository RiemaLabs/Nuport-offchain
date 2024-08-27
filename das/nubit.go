package das

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	nodeshare "github.com/RiemaLabs/nubit-node/da"
	blob "github.com/RiemaLabs/nubit-node/strucs/btx"
	verify "github.com/RiemaLabs/nuport-offchain/contractgen"
	"github.com/RiemaLabs/nuport-offchain/converter"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/spf13/pflag"

	nodeclient "github.com/RiemaLabs/nubit-node/rpc/rpc/client"

	nuport "github.com/RiemaLabs/nuport-offchain/nuport"
	"github.com/tendermint/tendermint/rpc/client/http"
)

type DAConfig struct {
	GasPrice        float64          `json:"gas-price"`
	GasMultiplier   float64          `json:"gas-multiplier"`
	Rpc             string           `json:"rpc"`
	NamespaceId     string           `json:"namespace-id" `
	AuthToken       string           `json:"auth-token"`
	NoopWriter      bool             `json:"noop-writer"`
	ValidatorConfig *ValidatorConfig `json:"validator-config"`
}

type ValidatorConfig struct {
	TendermintRPC string `json:"tendermint-rpc" reload:"hot"`
	EthClient     string `json:"eth-rpc" reload:"hot"`
	NuportAddr    string `json:"nuport"`
	VerifyAddr    string `json:"verify"`
}

var (
	nubitDALastSuccesfulActionGauge = metrics.NewRegisteredGauge("nubit/action/last_success", nil)
	nubitLastNonDefaultGasprice     = metrics.NewRegisteredGaugeFloat64("nubit/last_gas_price", nil)
	nubitSuccessCounter             = metrics.NewRegisteredCounter("nubit/action/nubit_success", nil)
	nubitFailureCounter             = metrics.NewRegisteredCounter("nubit/action/nubit_failure", nil)
	nubitGasRetries                 = metrics.NewRegisteredCounter("nubit/action/gas_retries", nil)
	nubitBlobInclusionRetries       = metrics.NewRegisteredCounter("nubit/action/inclusion_retries", nil)

	nubitValidationLastSuccesfulActionGauge = metrics.NewRegisteredGauge("nubit/validation/last_success", nil)
	nubitValidationSuccessCounter           = metrics.NewRegisteredCounter("nubit/validation/nuport_success", nil)
	nubitValidationFailureCounter           = metrics.NewRegisteredCounter("nubit/validation/nuport_failure", nil)
)

var (
	// ErrTxTimedout is the error message returned by the DA when mempool is congested
	ErrTxTimedout = errors.New("timed out waiting for tx to be included in a block")

	// ErrTxAlreadyInMempool is  the error message returned by the DA when tx is already in mempool
	ErrTxAlreadyInMempool = errors.New("tx already in mempool")

	// ErrTxIncorrectAccountSequence is the error message returned by the DA when tx has incorrect sequence
	ErrTxIncorrectAccountSequence = errors.New("incorrect account sequence")
)

type NubitDA struct {
	Cfg       *DAConfig
	Client    *nodeclient.Client
	Namespace *nodeshare.Namespace
	Prover    *NubitProver
}

type NubitProver struct {
	Trpc      *http.HTTP
	EthClient *ethclient.Client
	Nuport    *nuport.Bindings
	Verify    *verify.Bindings
}

// NubitMessageHeaderFlag indicates that this data is a Blob Pointer
// which will be used to retrieve data from Nubit
const NubitMessageHeaderFlag byte = 0x63

func NubitDAConfigAddOptions(prefix string, f *pflag.FlagSet) {
	f.Float64(prefix+".gas-price", 0.01, "Gas for retrying Nubit transactions")
	f.Float64(prefix+".gas-multiplier", 1.01, "Gas multiplier for Nubit transactions")
	f.String(prefix+".rpc", "", "Rpc endpoint for Nubit-node")
	f.String(prefix+".namespace-id", "", "Nubit Namespace to post data to")
	f.String(prefix+".auth-token", "", "Auth token for Nubit Node")
	f.Bool(prefix+".noop-writer", false, "Noop writer (disable posting to Nubit)")
	f.String(prefix+".validator-config"+".tendermint-rpc", "", "Tendermint RPC endpoint, only used for validation")
	f.String(prefix+".validator-config"+".eth-rpc", "", "L1 Websocket connection, only used for validation")
	f.String(prefix+".validator-config"+".nuport", "", "nuport address, only used for validation")
	f.String(prefix+".validator-config"+".verify", "", "Verify contract address, only used for validation")
}

// TODO: Add more logs for errors

func NewNubitDA(cfg *DAConfig, ethClient *ethclient.Client) (*NubitDA, error) {
	if cfg == nil {
		log.Error("Could not read Nubit DAConfig", "cfg", cfg)
		return nil, errors.New("nubit cfg cannot be blank")
	}
	daClient, err := nodeclient.NewClient(context.Background(), cfg.Rpc, cfg.AuthToken)
	if err != nil {
		log.Error("Could not create openrpc client", "err", err)
		return nil, err
	}

	if cfg.NamespaceId == "" {
		log.Error("Invalid Namespace ID", "namespace", cfg.NamespaceId)
		return nil, errors.New("namespace id cannot be blank")
	}

	namespace, err := nodeshare.NewBlobNamespaceV0([]byte(cfg.NamespaceId))
	if err != nil {
		log.Error("Invalid namespace ID", "err", err)
		return nil, err
	}
	log.Debug("Created namespace", "namespace", namespace)

	if cfg.ValidatorConfig != nil {
		trpc, err := http.New(cfg.ValidatorConfig.TendermintRPC, "/websocket")
		if err != nil {
			log.Error("Unable to create nubit-core tendermint rpc", "err", err)
			return nil, err
		}
		err = trpc.Start()
		if err != nil {
			log.Error("Unable to start nubit-core tendermint rpc", "err", err)
			return nil, err
		}

		var ethRpc *ethclient.Client
		if ethClient != nil {
			ethRpc = ethClient
		} else if len(cfg.ValidatorConfig.EthClient) > 0 {
			ethRpc, err = ethclient.Dial(cfg.ValidatorConfig.EthClient)
			if err != nil {
				log.Error("Unable to connect to eth rpc", "err", err)
				return nil, err
			}
		}

		nuport, err := nuport.NewBindings(common.HexToAddress(cfg.ValidatorConfig.NuportAddr), ethRpc)
		if err != nil {
			log.Error("Unable to create Nuport wrapper", "err", err)
			return nil, err
		}

		verifyC, err := verify.NewBindings(common.HexToAddress(cfg.ValidatorConfig.VerifyAddr), ethRpc)
		if err != nil {
			log.Error("Unable to create Verify contract wrapper", "err", err)
			return nil, err
		}

		return &NubitDA{
			Cfg:       cfg,
			Client:    daClient,
			Namespace: &namespace,
			Prover: &NubitProver{
				Trpc:      trpc,
				EthClient: ethRpc,
				Nuport:    nuport,
				Verify:    verifyC,
			},
		}, nil

	}

	return &NubitDA{
		Cfg:       cfg,
		Client:    daClient,
		Namespace: &namespace,
	}, nil
}

func (c *NubitDA) Stop() error {
	err := c.Prover.Trpc.Stop()
	if err != nil {
		log.Warn("Error stoping tendermint rpc client", "err", err)
		return err
	}
	c.Prover.EthClient.Close()
	c.Client.Close()
	return nil
}

func (c *NubitDA) Store(ctx context.Context, message []byte) ([]byte, error) {
	if c.Cfg.NoopWriter {
		log.Warn("NoopWriter enabled, falling back", "c.Cfg.NoopWriter", c.Cfg.NoopWriter)
		nubitFailureCounter.Inc(1)
		return nil, errors.New("NoopWriter enabled")
	}

	// set a 5 minute timeout context on submissions
	// if it takes longer than that to succesfully submit and verify a blob,
	// then there's an issue with the connection to the nubit node
	ctx, cancel := context.WithTimeout(ctx, time.Duration(time.Minute*5))
	defer cancel()
	dataBlob, err := blob.NewBlobV0(*c.Namespace, message)
	if err != nil {
		nubitFailureCounter.Inc(1)
		log.Warn("Error creating blob", "err", err)
		return nil, err
	}

	log.Debug("start submit")
	height := uint64(0)
	submitted := false
	// this will trigger node to use the default gas price from nubit app
	gasPrice := -1.0
	for !submitted {
		height, err = c.Client.Blob.Submit(ctx, []*blob.Blob{dataBlob} /*blob.GasPrice(gasPrice)*/, 0.01)
		if err != nil {
			switch {
			case strings.Contains(err.Error(), ErrTxTimedout.Error()), strings.Contains(err.Error(), ErrTxAlreadyInMempool.Error()), strings.Contains(err.Error(), ErrTxIncorrectAccountSequence.Error()):
				log.Warn("Failed to submit blob, bumping gas price and retrying...", "err", err)
				if gasPrice == -1.0 {
					gasPrice = c.Cfg.GasPrice
				} else {
					gasPrice = gasPrice * c.Cfg.GasMultiplier
				}

				nubitGasRetries.Inc(1)
				continue
			default:
				nubitFailureCounter.Inc(1)
				log.Warn("Blob Submission error", "err", err)
				return nil, err
			}
		}

		if height == 0 {
			nubitFailureCounter.Inc(1)
			log.Warn("Unexpected height from blob response", "height", height)
			return nil, errors.New("unexpected response code")
		}

		log.Debug("Created blob", "commitment", base64.StdEncoding.EncodeToString(dataBlob.Commitment), "namespace", base64.StdEncoding.EncodeToString(*c.Namespace), "height", height)

		submitted = true
		nubitLastNonDefaultGasprice.Update(gasPrice)
	}

	proofs, err := c.Client.Blob.GetProof(ctx, height, *c.Namespace, dataBlob.Commitment)
	if err != nil {
		nubitFailureCounter.Inc(1)
		log.Warn("Error retrieving proof", "err", err)
		return nil, err
	}

	proofRetries := 0
	for proofs == nil {
		log.Warn("Retrieved empty proof from GetProof, fetching again...", "proofRetries", proofRetries)
		time.Sleep(time.Millisecond * 100)
		proofs, err = c.Client.Blob.GetProof(ctx, height, *c.Namespace, dataBlob.Commitment)
		if err != nil {
			nubitFailureCounter.Inc(1)
			log.Warn("Error retrieving proof", "err", err)
			return nil, err
		}
		proofRetries++
		nubitBlobInclusionRetries.Inc(1)
	}

	included, err := c.Client.Blob.Included(ctx, height, *c.Namespace, proofs, dataBlob.Commitment)
	if err != nil || !included {
		nubitFailureCounter.Inc(1)
		log.Warn("Error checking for inclusion", "err", err, "proof", proofs)
		return nil, err
	}
	log.Info("Succesfully posted blob", "height", height, "commitment", hex.EncodeToString(dataBlob.Commitment))

	// we fetch the blob so that we can get the correct start index in the square
	dataBlob, err = c.Client.Blob.Get(ctx, height, *c.Namespace, dataBlob.Commitment)
	if err != nil {
		nubitFailureCounter.Inc(1)
		log.Warn("Error retrieving blob", "err", err)
		return nil, err
	}

	if dataBlob.Index() <= 0 {
		nubitFailureCounter.Inc(1)
		log.Warn("Unexpected index from blob response", "index", dataBlob.Index())
		return nil, errors.New("unexpected response code")
	}

	header, err := c.Client.Header.GetByHeight(ctx, height)
	if err != nil {
		nubitFailureCounter.Inc(1)
		log.Warn("Header retrieval error", "err", err)
		return nil, err
	}

	sharesLength := uint64(0)
	for _, proof := range *proofs {
		sharesLength += uint64(proof.End()) - uint64(proof.Start()) + 1
	}

	txCommitment, dataRoot := [32]byte{}, [32]byte{}
	copy(txCommitment[:], dataBlob.Commitment)

	copy(dataRoot[:], header.DataHash)

	// Row roots give us the length of the EDS
	squareSize := uint64(len(header.DAH.RowRoots))
	// ODS size
	odsSize := squareSize / 2

	blobIndex := uint64(dataBlob.Index())
	// startRow
	startRow := blobIndex / squareSize
	if odsSize*startRow > blobIndex {
		nubitFailureCounter.Inc(1)
		// return an empty batch
		return nil, fmt.Errorf("storing nubit information, odsSize*startRow=%v was larger than blobIndex=%v", odsSize*startRow, dataBlob.Index())
	}
	startIndexOds := blobIndex - odsSize*startRow
	blobPointer := BlobPointer{
		BlockHeight:  height,
		Start:        startIndexOds,
		SharesLength: sharesLength,
		TxCommitment: txCommitment,
		DataRoot:     dataRoot,
	}
	log.Info("Posted blob to height and dataRoot", "height", blobPointer.BlockHeight, "dataRoot", hex.EncodeToString(blobPointer.DataRoot[:]))

	blobPointerData, err := blobPointer.MarshalBinary()
	if err != nil {
		nubitFailureCounter.Inc(1)
		log.Warn("BlobPointer MashalBinary error", "err", err)
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, NubitMessageHeaderFlag)
	if err != nil {
		nubitFailureCounter.Inc(1)
		log.Warn("batch type byte serialization failed", "err", err)
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, blobPointerData)
	if err != nil {
		nubitFailureCounter.Inc(1)
		log.Warn("blob pointer data serialization failed", "err", err)
		return nil, err
	}

	serializedBlobPointerData := buf.Bytes()

	nubitSuccessCounter.Inc(1)
	nubitDALastSuccesfulActionGauge.Update(time.Now().Unix())

	return serializedBlobPointerData, nil
}

func (c *NubitDA) Read(ctx context.Context, blobPointer *BlobPointer) (*ReadResult, error) {
	// Wait until our client is synced
	err := c.Client.Header.SyncWait(ctx)
	if err != nil {
		return nil, err
	}

	header, err := c.Client.Header.GetByHeight(ctx, blobPointer.BlockHeight)
	if err != nil {
		return nil, err
	}

	headerDataHash := [32]byte{}
	copy(headerDataHash[:], header.DataHash)
	if headerDataHash != blobPointer.DataRoot {
		log.Error("Data Root mismatch", " header.DataHash", header.DataHash, "blobPointer.DataRoot", hex.EncodeToString(blobPointer.DataRoot[:]))
		return nil, fmt.Errorf("data root mismatch")
	}

	proofs, err := c.Client.Blob.GetProof(ctx, blobPointer.BlockHeight, *c.Namespace, blobPointer.TxCommitment[:])
	if err != nil {
		log.Error("Error retrieving proof", "err", err)
		return nil, err
	}

	sharesLength := uint64(0)
	for _, proof := range *proofs {
		sharesLength += uint64(proof.End()) - uint64(proof.Start()) + 1
	}

	if sharesLength != blobPointer.SharesLength {
		log.Error("Share length mismatch", "sharesLength", sharesLength, "blobPointer.SharesLength", blobPointer.SharesLength)
		return nil, fmt.Errorf("share length mismatch")
	}

	blob, err := c.Client.Blob.Get(ctx, blobPointer.BlockHeight, *c.Namespace, blobPointer.TxCommitment[:])
	if err != nil {
		// return an empty batch of data because we could not find the blob from the sequencer message
		log.Error("Failed to get blob", "height", blobPointer.BlockHeight, "commitment", hex.EncodeToString(blobPointer.TxCommitment[:]), "err", err)
		return nil, err
	}

	eds, err := c.Client.Share.GetEDS(ctx, header)
	if err != nil {
		log.Error("Failed to get EDS", "height", blobPointer.BlockHeight, "err", err)
		return nil, err
	}

	squareSize := uint64(eds.Width())
	odsSize := squareSize / 2

	startRow := blobPointer.Start / odsSize

	if blobPointer.Start >= odsSize*odsSize {
		log.Error("startIndexOds >= odsSize*odsSize", "startIndexOds", blobPointer.Start, "odsSize*odsSize", odsSize*odsSize)
		return nil, fmt.Errorf("startIndexOds >= odsSize*odsSize")
	}

	if blobPointer.Start+blobPointer.SharesLength < 1 {
		log.Error("startIndexOds+blobPointer.SharesLength < 1", "startIndexOds+blobPointer.SharesLength", blobPointer.Start+blobPointer.SharesLength)
		return nil, fmt.Errorf("startIndexOds+blobPointer.SharesLength < 1")
	}

	endIndexOds := blobPointer.Start + blobPointer.SharesLength - 1
	if endIndexOds >= odsSize*odsSize {
		log.Error("endIndexOds >= odsSize*odsSize", "endIndexOds", endIndexOds, "odsSize*odsSize", odsSize*odsSize)
		return nil, fmt.Errorf("endIndexOds >= odsSize*odsSize")
	}

	endRow := endIndexOds / odsSize

	if endRow >= odsSize || startRow >= odsSize {
		log.Error("endRow >= odsSize || startRow >= odsSize", "endRow", endRow, "startRow", startRow, "odsSize", odsSize)
		return nil, fmt.Errorf("endRow >= odsSize || startRow >= odsSize")
	}

	startColumn := blobPointer.Start % odsSize
	endColumn := endIndexOds % odsSize

	if startRow == endRow && startColumn > endColumn {
		log.Error("start and end row are the same and startColumn >= endColumn", "startColumn", startColumn, "endColumn+1 ", endColumn+1)
		return nil, fmt.Errorf("start and end row are the same and startColumn >= endColum")
	}

	rows := [][][]byte{}
	for i := startRow; i <= endRow; i++ {
		rows = append(rows, eds.Row(uint(i)))
	}

	return &ReadResult{
		Message:     blob.Data,
		RowRoots:    header.DAH.RowRoots,
		ColumnRoots: header.DAH.ColumnRoots,
		Rows:        rows,
		SquareSize:  squareSize,
		StartRow:    startRow,
		EndRow:      endRow,
	}, nil
}

func (c *NubitDA) filter(ctx context.Context, latestBlock uint64, nubitHeight uint64, backwards bool) (*nuport.BindingsDataCommitmentStored, error) {
	// Geth has a default of 5000 block limit for filters
	start := uint64(0)
	if latestBlock > 5000 {
		start = latestBlock - 5000
	}
	end := latestBlock

	for attempt := 0; attempt < 11; attempt++ {
		eventsIterator, err := c.Prover.Nuport.FilterDataCommitmentStored(
			&bind.FilterOpts{
				Context: ctx,
				Start:   start,
				End:     &end,
			},
			nil,
			nil,
			nil,
		)
		if err != nil {
			log.Error("Error creating event iterator", "err", err)
			return nil, err
		}

		var event *nuport.BindingsDataCommitmentStored
		for eventsIterator.Next() {
			e := eventsIterator.Event
			if e.StartBlock <= nubitHeight && nubitHeight < e.EndBlock {
				event = &nuport.BindingsDataCommitmentStored{
					ProofNonce:     e.ProofNonce,
					StartBlock:     e.StartBlock,
					EndBlock:       e.EndBlock,
					DataCommitment: e.DataCommitment,
				}
				break
			}
		}
		if err := eventsIterator.Error(); err != nil {
			log.Warn("Events Iterator error", "err", err)
			return nil, err
		}
		err = eventsIterator.Close()
		if err != nil {
			log.Warn("Error when closing Events Iterator error", "err", err)
			return nil, err
		}
		if event != nil {
			log.Info("Found Data Root submission event", "proof_nonce", event.ProofNonce, "start", event.StartBlock, "end", event.EndBlock)
			return event, nil
		}

		if backwards {
			if start >= 5000 {
				start -= 5000
			} else {
				start = 0
			}
			if end < 5000 {
				end = start + 1000
			} else {
				end -= 5000
			}
		} else {
			time.Sleep(time.Second * 3600)
			latestBlockNumber, err := c.Prover.EthClient.BlockNumber(context.Background())
			if err != nil {
				log.Warn("Error getting block number from Eth Client", "err", err)
				return nil, err
			}

			start = end
			end = latestBlockNumber
		}
	}

	return nil, fmt.Errorf("unable to find Data Commitment Stored event in nuport")
}

func (c *NubitDA) GetProof(ctx context.Context, msg []byte) ([]byte, error) {
	if c.Prover == nil {
		nubitValidationFailureCounter.Inc(1)
		return nil, fmt.Errorf("no nubit prover config found")
	}

	var flag byte
	buf := bytes.NewReader(msg)
	binary.Read(buf, binary.BigEndian, &flag)
	if flag != NubitMessageHeaderFlag {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Invalid nubit blob pointer header flag", "flag", flag)
		return nil, fmt.Errorf("invalid msg flag")
	}

	bpBinary := make([]byte, len(msg)-1)
	if err := binary.Read(buf, binary.BigEndian, &bpBinary); nil != err {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Couldn't read nubit blob pointer", "err", err)
		return nil, err
	}

	blobPointer := BlobPointer{}
	err := blobPointer.UnmarshalBinary(bpBinary)
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Couldn't unmarshal nubit blob pointer", "err", err)
		return nil, nil
	}

	latestBlockNumber, err := c.Prover.EthClient.BlockNumber(context.Background())
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Warn("Unable to retrieve latest block from EthClient", "err", err)
		return nil, err
	}

	// check the latest nubit block on the nuport contract
	latestNubitBlock, err := c.Prover.Nuport.LatestBlock(nil)
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Warn("Unable to retrieve latest block from nuport contract", "err", err)
		return nil, err
	}

	var backwards bool
	if blobPointer.BlockHeight < latestNubitBlock {
		backwards = true
	} else {
		backwards = false
		return nil, fmt.Errorf("blob pointer height is greater than latest nubit block")
	}

	var event *nuport.BindingsDataCommitmentStored

	event, err = c.filter(ctx, latestBlockNumber, blobPointer.BlockHeight, backwards)
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Warn("Unable to find event for DataCommitmentStored", "err", err)
		return nil, err
	}

	// get the block data root inclusion proof to the data root tuple root
	dataRootProof, err := c.Prover.Trpc.DataRootInclusionProof(ctx, blobPointer.BlockHeight, event.StartBlock, event.EndBlock)
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Warn("Unable to fetch DataRootInclusionProof", "err", err)
		return nil, err
	}

	// verify that the data root was committed to by the Nuport contract
	sideNodes := make([][32]byte, len(dataRootProof.Proof.Aunts))
	for i, aunt := range dataRootProof.Proof.Aunts {
		sideNodes[i] = *(*[32]byte)(aunt)
	}

	sharesProof, err := c.Prover.Trpc.ProveShares(ctx, blobPointer.BlockHeight, blobPointer.Start, blobPointer.Start+blobPointer.SharesLength)
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Unable to get ShareProof", "err", err)
		return nil, err
	}

	p := &Proof{
		SProof:   sharesProof,
		Nonce:    event.ProofNonce.Uint64(),
		Height:   blobPointer.BlockHeight,
		DataRoot: blobPointer.DataRoot,
		DRProof:  dataRootProof.Proof,
	}

	b, err := json.Marshal(p)
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Could not marshal proof", "err", err)
		return nil, err
	}

	return b, err
}

func (c *NubitDA) Verify(ctx context.Context, proof []byte) (bool, error) {
	p := &Proof{}
	if err := json.Unmarshal(proof, p); err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Could not unmarshal proof", "err", err)
		return false, err
	}

	if err := p.SProof.Validate(p.DataRoot[:]); nil != err {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Could not validate share proof", "err", err)
		return false, err
	}

	gGroof := converter.Conversion(p.SProof, p.Nonce, p.Height, p.DataRoot, p.DRProof)

	result, _, err := c.Prover.Verify.VerifySharesToDataRootTupleRoot(&bind.CallOpts{}, common.HexToAddress(c.Cfg.ValidatorConfig.NuportAddr), gGroof)
	if err != nil {
		nubitValidationFailureCounter.Inc(1)
		log.Error("Could not verify proof", "err", err)
		return false, err
	}

	return result, nil
}
