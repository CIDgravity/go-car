package index

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipld/go-car/v2/internal/carv1"
	"github.com/ipld/go-car/v2/internal/carv1/util"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-varint"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		codec   multicodec.Code
		want    Index
		wantErr bool
	}{
		{
			name:  "CarSortedIndexCodecIsConstructed",
			codec: multicodec.CarIndexSorted,
			want:  newSorted(),
		},
		{
			name:    "ValidMultiCodecButUnknwonToIndexIsError",
			codec:   multicodec.Cidv1,
			wantErr: true,
		},
		{
			name:    "IndexSingleSortedMultiCodecIsError",
			codec:   multicodec.Code(indexSingleSorted),
			wantErr: true,
		},
		{
			name:    "IndexHashedMultiCodecIsError",
			codec:   multicodec.Code(indexHashed),
			wantErr: true,
		},
		{
			name:    "IndexGobHashedMultiCodecIsError",
			codec:   multicodec.Code(indexGobHashed),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.codec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestReadFrom(t *testing.T) {
	idxf, err := os.Open("../testdata/sample-index.carindex")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, idxf.Close()) })

	subject, err := ReadFrom(idxf)
	require.NoError(t, err)

	crf, err := os.Open("../testdata/sample-v1.car")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, crf.Close()) })
	cr, err := carv1.NewCarReader(crf)
	require.NoError(t, err)

	for {
		wantBlock, err := cr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		// Get offset from the index for a CID and assert it exists
		gotOffset, err := subject.Get(wantBlock.Cid())
		require.NoError(t, err)
		require.NotZero(t, gotOffset)

		// Seek to the offset on CARv1 file
		_, err = crf.Seek(int64(gotOffset), io.SeekStart)
		require.NoError(t, err)

		// Read the fame at offset and assert the frame corresponds to the expected block.
		gotCid, gotData, err := util.ReadNode(crf)
		require.NoError(t, err)
		gotBlock, err := blocks.NewBlockWithCid(gotData, gotCid)
		require.NoError(t, err)
		require.Equal(t, wantBlock, gotBlock)
	}
}

func TestWriteTo(t *testing.T) {
	// Read sample index on file
	idxf, err := os.Open("../testdata/sample-index.carindex")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, idxf.Close()) })

	// Unmarshall to get expected index
	wantIdx, err := ReadFrom(idxf)
	require.NoError(t, err)

	// Write the same index out
	dest := filepath.Join(t.TempDir(), "index-write-to-test.carindex")
	destF, err := os.Create(dest)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, destF.Close()) })
	require.NoError(t, WriteTo(wantIdx, destF))

	// Seek to the beginning of the written out file.
	_, err = destF.Seek(0, io.SeekStart)
	require.NoError(t, err)

	// Read the written index back
	gotIdx, err := ReadFrom(destF)
	require.NoError(t, err)

	// Assert they are equal
	require.Equal(t, wantIdx, gotIdx)
}

func TestSave(t *testing.T) {
	// Read sample index on file
	idxf, err := os.Open("../testdata/sample-index.carindex")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, idxf.Close()) })

	// Unmarshall to get expected index
	wantIdx, err := ReadFrom(idxf)
	require.NoError(t, err)

	// Save the same index at destination
	dest := filepath.Join(t.TempDir(), "index-write-to-test.carindex")
	require.NoError(t, Save(wantIdx, dest))

	// Open the saved file
	destF, err := os.Open(dest)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, destF.Close()) })

	// Read the written index back
	gotIdx, err := ReadFrom(destF)
	require.NoError(t, err)

	// Assert they are equal
	require.Equal(t, wantIdx, gotIdx)
}

func TestMarshalledIndexStartsWithCodec(t *testing.T) {
	// Read sample index on file
	idxf, err := os.Open("../testdata/sample-index.carindex")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, idxf.Close()) })

	// Unmarshall to get expected index
	wantIdx, err := ReadFrom(idxf)
	require.NoError(t, err)

	// Assert the first two bytes are the corresponding multicodec code.
	buf := new(bytes.Buffer)
	require.NoError(t, WriteTo(wantIdx, buf))
	require.Equal(t, varint.ToUvarint(uint64(multicodec.CarIndexSorted)), buf.Bytes()[:2])
}
