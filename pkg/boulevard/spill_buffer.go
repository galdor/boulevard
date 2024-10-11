package boulevard

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"go.n16f.net/boulevard/pkg/netutils"
)

var (
	ErrSpillBufferFull = errors.New("maximum buffer capacity reached")
)

type SpillBuffer struct {
	buffer []byte
	file   *os.File

	size int64

	filePath      string
	maxMemorySize int64
	maxBufferSize int64
}

func NewSpillBuffer(filePath string, maxMemorySize, maxBufferSize int64) *SpillBuffer {
	buf := SpillBuffer{
		filePath:      filePath,
		maxMemorySize: maxMemorySize,
		maxBufferSize: maxBufferSize,
	}

	return &buf
}

func (buf *SpillBuffer) Size() int64 {
	return buf.size
}

func (buf *SpillBuffer) Close() error {
	if buf.file != nil {
		if err := buf.file.Close(); err != nil {
			return err
		}

		buf.file = nil

		if err := os.Remove(buf.filePath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("cannot delete %q: %w", buf.filePath, err)
			}
		}
	}

	return nil
}

func (buf *SpillBuffer) Reader() (io.ReadCloser, error) {
	if buf.file == nil {
		r := bytes.NewReader(buf.buffer)
		return io.NopCloser(r), nil
	}

	file, err := os.Open(buf.filePath)
	if err != nil {
		err = netutils.UnwrapOpError(err, "open")
		return nil, fmt.Errorf("cannot open %q: %w", buf.filePath, err)
	}

	return file, nil
}

func (buf *SpillBuffer) Write(data []byte) (int, error) {
	if buf.size+int64(len(data)) > buf.maxBufferSize {
		return 0, ErrSpillBufferFull
	}

	if buf.file == nil {
		if int64(len(buf.buffer))+int64(len(data)) <= buf.maxMemorySize {
			buf.buffer = append(buf.buffer, data...)
			buf.size += int64(len(data))
			return len(data), nil
		}

		flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		file, err := os.OpenFile(buf.filePath, flags, 0600)
		if err != nil {
			err = netutils.UnwrapOpError(err, "open")
			return 0, fmt.Errorf("cannot open %q: %w", buf.filePath, err)
		}

		buf.file = file

		if _, err := buf.file.Write(buf.buffer); err != nil {
			err = netutils.UnwrapOpError(err, "write")
			return 0, fmt.Errorf("cannot write %q: %w", buf.filePath, err)
		}

		buf.buffer = nil
	}

	n, err := buf.file.Write(data)
	if err != nil {
		err = netutils.UnwrapOpError(err, "write")
		return n, fmt.Errorf("cannot write %q: %w", buf.filePath, err)
	}

	buf.size += int64(n)

	return n, nil
}
