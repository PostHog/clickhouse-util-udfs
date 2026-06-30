package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessLineRemovesEmptyStrings(t *testing.T) {
	tests := map[string]string{
		`{"a":"","b":"x","c":null}`:                           `{"a":null,"b":"x","c":null}`,
		`{"a":"","a":"x","nested":{"b":""}}`:                  `{"a":null,"a":"x","nested":{"b":null}}`,
		`{"arr":["",{"a":""},"x"],"obj.x":"","obj":{"x":""}}`: `{"arr":[null,{"a":null},"x"],"obj.x":null,"obj":{"x":null}}`,
	}

	for input, want := range tests {
		var buf bytes.Buffer
		if err := processLine([]byte(input), &buf); err != nil {
			t.Fatalf("processLine(%s) returned error: %v", input, err)
		}
		if got := buf.String(); got != want {
			t.Fatalf("processLine(%s) = %s, want %s", input, got, want)
		}
	}
}

func TestProcessLineErrorsOnMalformedJSON(t *testing.T) {
	var buf bytes.Buffer
	err := processLine([]byte("{\"a\":"), &buf)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestShouldStringifyNumber(t *testing.T) {
	tests := map[string]bool{
		"0":                    false,
		"42":                   false,
		"9223372036854775807":  false,
		"9223372036854775808":  true,
		"-9223372036854775808": false,
		"-9223372036854775809": true,
		"1.25":                 false,
		"1e6":                  false,
	}

	for input, want := range tests {
		if got := shouldStringifyNumber(input); got != want {
			t.Fatalf("shouldStringifyNumber(%q) = %v, want %v", input, got, want)
		}
	}
}

func BenchmarkProcessLine(b *testing.B) {
	input := []byte(`{"id":1,"empty":"","nested":{"empty":"","value":"x"},"arr":["",{"empty":""}],"amount":934504962295726700000}`)
	var buf bytes.Buffer

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		if err := processLine(input, &buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessFixture(b *testing.B) {
	lines, totalBytes := loadBenchmarkLines(b)
	var buf bytes.Buffer

	b.ReportAllocs()
	b.SetBytes(int64(totalBytes))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, line := range lines {
			if err := processLine(line, &buf); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func loadBenchmarkLines(b *testing.B) ([][]byte, int) {
	b.Helper()

	path := os.Getenv("BENCH_FILE")
	if path == "" {
		return generatedBenchmarkLines()
	} else if !filepath.IsAbs(path) {
		path = filepath.Join("../..", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}

	rawLines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	lines := make([][]byte, 0, len(rawLines))
	totalBytes := 0
	for _, line := range rawLines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		lines = append(lines, line)
		totalBytes += len(line)
	}

	if len(lines) == 0 {
		b.Fatalf("benchmark file has no JSON lines: %s", path)
	}

	return lines, totalBytes
}

func generatedBenchmarkLines() ([][]byte, int) {
	lines := make([][]byte, 0, 256)
	totalBytes := 0
	for i := 0; i < 256; i++ {
		value := `""`
		if i%3 == 0 {
			value = fmt.Sprintf("%q", fmt.Sprintf("value-%d", i))
		}

		line := []byte(fmt.Sprintf(
			`{"id":%d,"empty":%s,"nested":{"empty":"","value":"v%d"},"arr":["",{"empty":%s}],"amount":934504962295726700000}`,
			i,
			value,
			i,
			value,
		))
		lines = append(lines, line)
		totalBytes += len(line)
	}

	return lines, totalBytes
}
