package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessLineCleansEventProperties(t *testing.T) {
	input := []byte(`{"$active_feature_flags":"undefined","$active_feature_flags":["beta",42,null,{"a.b":1}],"Account.client_id":"abc","Account":{"client_id":null},"huge":18446744073709551616,"max_uint":18446744073709551615,"too_negative":-9223372036854775809,"min_int":-9223372036854775808,"null_field":null,"dupe":"","dupe":"kept","emptydupe":"","emptydupe":null}`)
	want := `{"$active_feature_flags":["beta","42","","{\"a\":{\"b\":1}}"],"Account":{"client_id":"abc"},"huge":"18446744073709551616","max_uint":18446744073709551615,"too_negative":"-9223372036854775809","min_int":-9223372036854775808,"dupe":"kept","emptydupe":""}`

	var got bytes.Buffer
	if err := processLine(input, &got); err != nil {
		t.Fatal(err)
	}
	if got.String() != want {
		t.Fatalf("processLine() = %s, want %s", got.String(), want)
	}
}

func TestProcessLineParsesStringifiedArrayPath(t *testing.T) {
	input := []byte(`{"$exception_types":"[\"TypeError\",7,null,{\"x.y\":\"z\"}]"}`)
	want := `{"$exception_types":["TypeError","7","","{\"x\":{\"y\":\"z\"}}"]}`

	var got bytes.Buffer
	if err := processLine(input, &got); err != nil {
		t.Fatal(err)
	}
	if got.String() != want {
		t.Fatalf("processLine() = %s, want %s", got.String(), want)
	}
}

func TestProcessLineCoercesArrayPathScalars(t *testing.T) {
	tests := map[string]string{
		`{"$exception_sources":"undefined"}`:         `{"$exception_sources":[]}`,
		`{"$exception_sources":"worker"}`:            `{"$exception_sources":["worker"]}`,
		`{"$exception_sources":false}`:               `{"$exception_sources":["false"]}`,
		`{"$exception_sources":{}}`:                  `{"$exception_sources":[]}`,
		`{"$exception_sources":{"worker.id":3}}`:     `{"$exception_sources":["{\"worker\":{\"id\":3}}"]}`,
		`{"nested":{"$exception_sources":"worker"}}`: `{"nested":{"$exception_sources":"worker"}}`,
	}

	for input, want := range tests {
		var got bytes.Buffer
		if err := processLine([]byte(input), &got); err != nil {
			t.Fatalf("processLine(%s) returned error: %v", input, err)
		}
		if got.String() != want {
			t.Fatalf("processLine(%s) = %s, want %s", input, got.String(), want)
		}
	}
}

func TestCleanNodeMatchesNestedArrayStringPath(t *testing.T) {
	input := []byte(`{"outer":[{"$exception_sources":"undefined"},{"$exception_sources":"worker"}],"nested":{"$exception_sources":"worker"}}`)
	want := `{"outer":[{"$exception_sources":[]},{"$exception_sources":["worker"]}],"nested":{"$exception_sources":"worker"}}`

	var proc processor
	proc.data = input
	parsed, err := proc.parseValue()
	if err != nil {
		t.Fatal(err)
	}
	cleaned, err := proc.cleanNode(makePathRules("outer.$exception_sources"), parsed)
	if err != nil {
		t.Fatal(err)
	}
	defer proc.recycle(cleaned)

	var got bytes.Buffer
	proc.writeValue(&got, cleaned)
	if got.String() != want {
		t.Fatalf("cleanNode() = %s, want %s", got.String(), want)
	}
}

func TestProcessLineHandlesEscapedDottedKeysAndStrings(t *testing.T) {
	input := []byte("{\"a\\u002eb\":\"line\\nquote\\\"\",\"emoji\":\"\\ud83d\\ude00\"}")
	want := "{\"a\":{\"b\":\"line\\nquote\\\"\"},\"emoji\":\"\U0001F600\"}"

	var got bytes.Buffer
	if err := processLine(input, &got); err != nil {
		t.Fatal(err)
	}
	if got.String() != want {
		t.Fatalf("processLine() = %s, want %s", got.String(), want)
	}
}

func TestProcessLineErrorsOnMalformedJSON(t *testing.T) {
	var got bytes.Buffer
	if err := processLine([]byte(`{"broken"`), &got); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestShouldStringifyNumber(t *testing.T) {
	tests := map[string]bool{
		"18446744073709551615":  false,
		"18446744073709551616":  true,
		"9223372036854775808":   false,
		"-9223372036854775808":  false,
		"-9223372036854775809":  true,
		"1.8446744073709552e19": false,
		"42":                    false,
	}

	for input, want := range tests {
		if got := shouldStringifyNumber(input); got != want {
			t.Fatalf("shouldStringifyNumber(%q) = %v, want %v", input, got, want)
		}
	}
}

func BenchmarkProcessLine(b *testing.B) {
	input := []byte(`{"$active_feature_flags":"[\"beta\", \"new-ui\"]","$exception_types":"undefined","Account.client_id":"client_123","huge":18446744073709551616,"small":123,"dotted.key":"value","duplicate":"","duplicate":"kept","null_field":null,"nested":{"a.b":{"c":1}}}`)
	var buf bytes.Buffer
	proc := processor{}

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		if err := proc.processLine(input, &buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessFixture(b *testing.B) {
	lines, totalBytes := loadBenchmarkLines(b)
	var buf bytes.Buffer
	proc := processor{}

	b.ReportAllocs()
	b.SetBytes(int64(totalBytes))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, line := range lines {
			if err := proc.processLine(line, &buf); err != nil {
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
		line := []byte(fmt.Sprintf(
			`{"$active_feature_flags":"[\"beta-%d\"]","$exception_types":"undefined","Account.client_id":"client_%d","huge":18446744073709551616,"duplicate":"","duplicate":"kept","nested":{"a.b":{"c":%d}}}`,
			i,
			i,
			i,
		))
		lines = append(lines, line)
		totalBytes += len(line)
	}
	return lines, totalBytes
}
