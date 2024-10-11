package boulevard

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpillBuffer(t *testing.T) {
	require := require.New(t)

	dirPath := t.TempDir()
	filePath := path.Join(dirPath, "1")

	buf := NewSpillBuffer(filePath, 5, 8)
	defer buf.Close()

	n, err := buf.Write([]byte("foo"))
	require.Equal(3, n)
	require.NoError(err)
	require.Equal([]byte("foo"), buf.buffer)
	require.Nil(buf.file)

	n, err = buf.Write([]byte("bar"))
	require.Equal(3, n)
	require.NoError(err)
	require.Nil(buf.buffer)
	require.NotNil(buf.file)
	data, err := os.ReadFile(buf.filePath)
	require.NoError(err)
	require.Equal([]byte("foobar"), data)

	n, err = buf.Write([]byte("baz"))
	require.Equal(0, n)
	require.Error(err)
	require.ErrorIs(err, ErrSpillBufferFull)
	data, err = os.ReadFile(buf.filePath)
	require.NoError(err)
	require.Equal([]byte("foobar"), data)
}
