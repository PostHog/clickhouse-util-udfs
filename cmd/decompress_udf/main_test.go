package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

func TestDecompress(t *testing.T) {
	input := []byte{'h', 'i', 0, '\n', '\t', 0xff}
	fixtures := compressedFixtures(t, input)
	proc, err := newProcessor(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer proc.close()

	for codec, compressed := range fixtures {
		for _, requested := range []string{strings.ToLower(codec), ""} {
			if codec == "LZ4Block" && requested == "" {
				continue
			}
			got, err := proc.decompress(compressed, requested)
			if err != nil {
				t.Fatalf("decompress(%s, %q): %v", codec, requested, err)
			}
			if !bytes.Equal(got, input) {
				t.Fatalf("decompress(%s, %q) = %x, want %x", codec, requested, got, input)
			}
		}
	}

	for codec, compressed := range compressedFixtures(t, nil) {
		if codec == "LZ4Block" {
			continue
		}
		got, err := proc.decompress(compressed, "")
		if err != nil || len(got) != 0 {
			t.Fatalf("decompress empty %s = %x, %v", codec, got, err)
		}
	}
}

func TestDecompressFailsFast(t *testing.T) {
	fixtures := compressedFixtures(t, []byte("hello"))
	proc, err := newProcessor(4)
	if err != nil {
		t.Fatal(err)
	}
	defer proc.close()

	tests := []struct {
		name  string
		data  []byte
		codec string
	}{
		{"unknown header", []byte("plain"), ""},
		{"raw block detection", fixtures["LZ4Block"], ""},
		{"unsupported codec", fixtures["GZIP"], "SNAPPY"},
		{"codec mismatch", fixtures["GZIP"], "ZSTD"},
		{"output limit", fixtures["GZIP"], "GZIP"},
		{"truncated stream", fixtures["ZSTD"][:5], "ZSTD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := proc.decompress(test.data, test.codec); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestRowBinaryTransport(t *testing.T) {
	value := []byte{'a', '\n', 0, 0xff}
	fixtures := compressedFixtures(t, value)
	var input bytes.Buffer
	writer := bufio.NewWriter(&input)
	for _, codec := range []string{"GZIP", "ZSTD", "LZ4", "LZ4Block"} {
		if err := writeString(writer, fixtures[codec]); err != nil {
			t.Fatal(err)
		}
		if err := writeString(writer, []byte(codec)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := run(&input, &output, 1024); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	for range 4 {
		got, err := readString(reader)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, value) {
			t.Fatalf("got %x, want %x", got, value)
		}
	}
}

func TestRowBinaryRejectsTruncatedInput(t *testing.T) {
	if err := run(bytes.NewReader([]byte{1}), &bytes.Buffer{}, 1024); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDetectCodecSkipsMetadataFrame(t *testing.T) {
	fixtures := compressedFixtures(t, []byte("hello"))
	proc, err := newProcessor(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer proc.close()
	for _, codec := range []string{"ZSTD", "LZ4"} {
		data := append([]byte{0x50, 0x2a, 0x4d, 0x18, 0x03, 0, 0, 0, 'a', 'b', 'c'}, fixtures[codec]...)
		if got, err := detectCodec(data); err != nil || got != codec {
			t.Fatalf("detectCodec() = %q, %v, want %s", got, err, codec)
		}
		if got, err := proc.decompress(data, ""); err != nil || string(got) != "hello" {
			t.Fatalf("decompress(%s) = %q, %v", codec, got, err)
		}
	}
}

func compressedFixtures(t *testing.T, input []byte) map[string][]byte {
	t.Helper()
	fixtures := make(map[string][]byte)

	var gzipData bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipData)
	if _, err := gzipWriter.Write(input); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	fixtures["GZIP"] = gzipData.Bytes()

	zstdWriter, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	fixtures["ZSTD"] = zstdWriter.EncodeAll(input, nil)
	zstdWriter.Close()

	var lz4Data bytes.Buffer
	lz4Writer := lz4.NewWriter(&lz4Data)
	if _, err := lz4Writer.Write(input); err != nil {
		t.Fatal(err)
	}
	if err := lz4Writer.Close(); err != nil {
		t.Fatal(err)
	}
	fixtures["LZ4"] = lz4Data.Bytes()

	block := make([]byte, lz4.CompressBlockBound(len(input)))
	n, err := lz4.CompressBlock(input, block, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("test input is not compressible as an LZ4 block: %s", hex.EncodeToString(input))
	}
	fixtures["LZ4Block"] = block[:n]
	return fixtures
}
