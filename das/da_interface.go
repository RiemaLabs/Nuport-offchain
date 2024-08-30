package das

import (
	"context"
)

type NubitWriter interface {
	Store(context.Context, []byte) ([]byte, error)
}

type WithProof interface {
	Proof(context.Context, []byte) ([]byte, error)
}

type ReadResult struct {
	Message     []byte     `json:"message"`
	RowRoots    [][]byte   `json:"row_roots"`
	ColumnRoots [][]byte   `json:"column_roots"`
	Rows        [][][]byte `json:"rows"`
	SquareSize  uint64     `json:"square_size"` // Refers to original data square size
	StartRow    uint64     `json:"start_row"`
	EndRow      uint64     `json:"end_row"`
}

type NubitReader interface {
	Read(ctx context.Context, blobPointer *BlobPointer) (*ReadResult, error)
	GetProof(ctx context.Context, msg []byte) ([]byte, error)
	Verify(ctx context.Context, msg []byte) (bool, error)
}
