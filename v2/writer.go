package car

import (
	"bytes"
	"context"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	carv1 "github.com/ipld/go-car"
	"github.com/ipld/go-car/v2/carbs"
	"io"
)

const bulkPaddingBytesSize = 1024

var bulkPadding = make([]byte, bulkPaddingBytesSize)

type (
	// padding represents the number of padding bytes.
	padding uint64
	// Writer writes CAR v2 into a give io.Writer.
	Writer struct {
		Walk         carv1.WalkFunc
		IndexCodec   carbs.IndexCodec
		NodeGetter   format.NodeGetter
		CarV1Padding uint64
		IndexPadding uint64

		ctx          context.Context
		roots        []cid.Cid
		encodedCarV1 *bytes.Buffer
	}
)

// WriteTo writes this padding to the given writer as default value bytes.
func (p padding) WriteTo(w io.Writer) (n int64, err error) {
	var reminder int64
	if p > bulkPaddingBytesSize {
		reminder = int64(p % bulkPaddingBytesSize)
		iter := int(p / bulkPaddingBytesSize)
		for i := 0; i < iter; i++ {
			if _, err = w.Write(bulkPadding); err != nil {
				return
			}
			n += bulkPaddingBytesSize
		}
	} else {
		reminder = int64(p)
	}

	paddingBytes := make([]byte, reminder)
	_, err = w.Write(paddingBytes)
	n += reminder
	return
}

// NewWriter instantiates a new CAR v2 writer.
// The writer instantiated uses `carbs.IndexSorted` as the index codec,
// and `carv1.DefaultWalkFunc` as the default walk function.
func NewWriter(ctx context.Context, ng format.NodeGetter, roots []cid.Cid) *Writer {
	return &Writer{
		Walk:         carv1.DefaultWalkFunc,
		IndexCodec:   carbs.IndexSorted,
		NodeGetter:   ng,
		ctx:          ctx,
		roots:        roots,
		encodedCarV1: new(bytes.Buffer),
	}
}

// WriteTo writes the given root CIDs according to CAR v2 specification, traversing the DAG using the
// Writer.Walk function.
func (w *Writer) WriteTo(writer io.Writer) (n int64, err error) {
	_, err = writer.Write(PrefixBytes)
	if err != nil {
		return
	}
	n += int64(prefixBytesSize)
	// We read the entire car into memory because carbs.GenerateIndex takes a reader.
	// Future PRs will make this more efficient by exposing necessary interfaces in carbs so that
	// this can be done in an streaming manner.
	if err = carv1.WriteCarWithWalker(w.ctx, w.NodeGetter, w.roots, w.encodedCarV1, w.Walk); err != nil {
		return
	}
	carV1Len := w.encodedCarV1.Len()

	wn, err := w.writeHeader(writer, carV1Len)
	if err != nil {
		return
	}
	n += wn

	wn, err = padding(w.CarV1Padding).WriteTo(writer)
	if err != nil {
		return
	}
	n += wn

	carV1Bytes := w.encodedCarV1.Bytes()
	wwn, err := writer.Write(carV1Bytes)
	if err != nil {
		return
	}
	n += int64(wwn)

	wn, err = padding(w.IndexPadding).WriteTo(writer)
	if err != nil {
		return
	}
	n += wn

	wn, err = w.writeIndex(writer, carV1Bytes)
	if err == nil {
		n += wn
	}
	return
}

func (w *Writer) writeHeader(writer io.Writer, carV1Len int) (int64, error) {
	header := NewHeader(uint64(carV1Len)).
		WithCarV1Padding(w.CarV1Padding).
		WithIndexPadding(w.IndexPadding)
	return header.WriteTo(writer)
}

func (w *Writer) writeIndex(writer io.Writer, carV1 []byte) (n int64, err error) {
	// TODO avoid recopying the bytes by refactoring carbs once it is integrated here.
	// Right now we copy the bytes since carbs takes a writer.
	// Consider refactoring carbs to make this process more efficient.
	// We should avoid reading the entire car into memory since it can be large.
	reader := bytes.NewReader(carV1)
	index, err := carbs.GenerateIndex(reader, int64(len(carV1)), carbs.IndexSorted, true)
	if err != nil {
		return
	}
	err = index.Marshal(writer)
	// FIXME refactor carbs to expose the number of bytes written.
	return
}