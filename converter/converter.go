package converter

import (
	"math/big"

	verify "github.com/RiemaLabs/nuport-offchain/contractgen"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

func Conversion(
	proof types.ShareProof,
	nonce uint64,
	height uint64,
	blockDataRoot [32]byte,
	dataRootInclusionProof merkle.Proof,
) verify.SharesProof {
	var result verify.SharesProof
	result.Data = proof.Data
	result.ShareProofs = make([]verify.NamespaceMerkleMultiproof, 0)
	//TODO: InclusionProof conversion
	result.Namespace = *namespace(proof.NamespaceID)
	result.RowRoots = toRowRoots(proof.RowProof.RowRoots)
	result.RowProofs = toRowProofs(proof.RowProof.Proofs)
	result.AttestationProof = toAttestationProof(nonce, height, blockDataRoot, dataRootInclusionProof)
	return result
}

func toRowRoots(roots []tmbytes.HexBytes) []verify.NamespaceNode {
	rowRoots := make([]verify.NamespaceNode, len(roots))
	for i, root := range roots {
		rowRoots[i] = *toNamespaceNode(root.Bytes())
	}
	return rowRoots
}

func minNamespace(innerNode []byte) *verify.Namespace {
	version := innerNode[0]
	var id [28]byte
	copy(id[:], innerNode[1:29])
	return &verify.Namespace{
		Version: [1]byte{version},
		Id:      id,
	}
}

func maxNamespace(innerNode []byte) *verify.Namespace {
	version := innerNode[29]
	var id [28]byte
	copy(id[:], innerNode[30:58])
	return &verify.Namespace{
		Version: [1]byte{version},
		Id:      id,
	}
}

func namespace(namespaceID []byte) *verify.Namespace {
	version := namespaceID[0]
	var id [28]byte
	for i, b := range namespaceID[1:] {
		id[i] = b
	}
	return &verify.Namespace{
		Version: [1]byte{version},
		Id:      id,
	}
}

func toNamespaceNode(node []byte) *verify.NamespaceNode {
	minNs := minNamespace(node)
	maxNs := maxNamespace(node)
	digest := make([]byte, len(node)-58)
	for i, b := range node[58:] {
		digest[i] = b
	}
	return &verify.NamespaceNode{
		Min:    *minNs,
		Max:    *maxNs,
		Digest: digest,
	}
}

func toRowProofs(proofs []*merkle.Proof) []verify.BinaryMerkleProof {
	rowProofs := make([]verify.BinaryMerkleProof, len(proofs))
	for i, proof := range proofs {
		sideNodes := make([][32]byte, len(proof.Aunts))
		for j, sideNode := range proof.Aunts {
			var bzSideNode [32]byte
			copy(bzSideNode[:], sideNode)
			sideNodes[j] = bzSideNode
		}
		rowProofs[i] = verify.BinaryMerkleProof{
			SideNodes: sideNodes,
			Key:       big.NewInt(proof.Index),
			NumLeaves: big.NewInt(proof.Total),
		}
	}
	return rowProofs
}

func toAttestationProof(
	nonce uint64,
	height uint64,
	blockDataRoot [32]byte,
	dataRootInclusionProof merkle.Proof,
) verify.AttestationProof {
	sideNodes := make([][32]byte, len(dataRootInclusionProof.Aunts))
	for i, sideNode := range dataRootInclusionProof.Aunts {
		if len(sideNode) != 32 {
			panic("Invalid aunt")
		}
		var bzSideNode [32]byte
		copy(bzSideNode[:], sideNode)
		sideNodes[i] = bzSideNode
	}

	return verify.AttestationProof{
		TupleRootNonce: big.NewInt(int64(nonce)),
		Tuple: verify.DataRootTuple{
			Height:   big.NewInt(int64(height)),
			DataRoot: blockDataRoot,
		},
		Proof: verify.BinaryMerkleProof{
			SideNodes: sideNodes,
			Key:       big.NewInt(dataRootInclusionProof.Index),
			NumLeaves: big.NewInt(dataRootInclusionProof.Total),
		},
	}
}
