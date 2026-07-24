package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

const maxDecompressedSize = 64 << 20

type processor struct {
	maxOutput int
	zstd      *zstd.Decoder
	output    bytes.Buffer
	lz4Block  []byte
}

func newProcessor(maxOutput int) (*processor, error) {
	memoryLimit := max(maxOutput, zstd.MinWindowSize)
	decoder, err := zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxMemory(uint64(memoryLimit)),
	)
	if err != nil {
		return nil, fmt.Errorf("create ZSTD decoder: %w", err)
	}
	return &processor{maxOutput: maxOutput, zstd: decoder}, nil
}

func (p *processor) close() {
	p.zstd.Close()
}

func (p *processor) decompress(data []byte, codec string) ([]byte, error) {
	codec = strings.ToUpper(codec)
	if codec == "" {
		var err error
		codec, err = detectCodec(data)
		if err != nil {
			return nil, err
		}
	}

	switch codec {
	case "GZIP":
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("invalid GZIP stream: %w", err)
		}
		decompressed, err := p.readLimited(reader)
		_ = reader.Close()
		return decompressed, err
	case "ZSTD":
		if err := p.zstd.Reset(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("invalid ZSTD stream: %w", err)
		}
		return p.readLimited(p.zstd)
	case "LZ4":
		return p.readLimited(lz4.NewReader(bytes.NewReader(data)))
	case "LZ4BLOCK":
		if p.lz4Block == nil {
			p.lz4Block = make([]byte, p.maxOutput)
		}
		n, err := lz4.UncompressBlock(data, p.lz4Block)
		if err != nil {
			return nil, fmt.Errorf("invalid LZ4Block or decompressed size exceeds %d bytes: %w", p.maxOutput, err)
		}
		return p.lz4Block[:n], nil
	default:
		return nil, fmt.Errorf("unsupported compression codec %q", codec)
	}
}

func (p *processor) readLimited(reader io.Reader) ([]byte, error) {
	p.output.Reset()
	n, err := io.CopyN(&p.output, reader, int64(p.maxOutput)+1)
	if n > int64(p.maxOutput) {
		return nil, fmt.Errorf("decompressed size exceeds %d bytes", p.maxOutput)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}
	return p.output.Bytes(), nil
}

func detectCodec(data []byte) (string, error) {
	remaining := data
	for len(remaining) >= 8 && binary.LittleEndian.Uint32(remaining[:4])&0xfffffff0 == 0x184d2a50 {
		skip := uint64(binary.LittleEndian.Uint32(remaining[4:8])) + 8
		if skip > uint64(len(remaining)) {
			return "", errors.New("truncated skippable compression frame")
		}
		remaining = remaining[int(skip):]
	}

	switch {
	case len(remaining) >= 3 && bytes.Equal(remaining[:3], []byte{0x1f, 0x8b, 0x08}):
		return "GZIP", nil
	case len(remaining) >= 4 && bytes.Equal(remaining[:4], []byte{0x28, 0xb5, 0x2f, 0xfd}):
		return "ZSTD", nil
	case len(remaining) >= 4 && bytes.Equal(remaining[:4], []byte{0x04, 0x22, 0x4d, 0x18}):
		return "LZ4", nil
	default:
		return "", errors.New("cannot detect compression codec from frame header")
	}
}

func readString(reader *bufio.Reader) ([]byte, error) {
	length, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}
	if length > uint64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("RowBinary string length %d exceeds platform limit", length)
	}
	value := make([]byte, int(length))
	if _, err := io.ReadFull(reader, value); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return value, nil
}

func writeString(writer *bufio.Writer, value []byte) error {
	var length [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(length[:], uint64(len(value)))
	if _, err := writer.Write(length[:n]); err != nil {
		return err
	}
	_, err := writer.Write(value)
	return err
}

func run(input io.Reader, output io.Writer, maxOutput int) error {
	proc, err := newProcessor(maxOutput)
	if err != nil {
		return err
	}
	defer proc.close()

	reader := bufio.NewReaderSize(input, 4*1024*1024)
	writer := bufio.NewWriterSize(output, 4*1024*1024)
	for {
		data, err := readString(reader)
		if errors.Is(err, io.EOF) {
			return writer.Flush()
		}
		if err != nil {
			return fmt.Errorf("read compressed data: %w", err)
		}
		codec, err := readString(reader)
		if err != nil {
			return fmt.Errorf("read codec: %w", err)
		}
		decompressed, err := proc.decompress(data, string(codec))
		if err != nil {
			return err
		}
		if err := writeString(writer, decompressed); err != nil {
			return fmt.Errorf("write decompressed data: %w", err)
		}
	}
}

func main() {
	if err := run(os.Stdin, os.Stdout, maxDecompressedSize); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
