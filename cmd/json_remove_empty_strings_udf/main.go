package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/pprof"

	"github.com/valyala/fastjson"
)

var nullValue = fastjson.MustParse("null")

type processor struct {
	parser        fastjson.Parser
	arena         fastjson.Arena
	numberScratch []byte
}

func removeEmptyStrings(value *fastjson.Value, arena *fastjson.Arena, numberScratch *[]byte) {
	switch value.Type() {
	case fastjson.TypeObject:
		obj, _ := value.Object()
		obj.Visit(func(_ []byte, v *fastjson.Value) {
			removeEmptyStrings(v, arena, numberScratch)
		})
	case fastjson.TypeArray:
		values, _ := value.Array()
		for _, child := range values {
			removeEmptyStrings(child, arena, numberScratch)
		}
	case fastjson.TypeString:
		if len(value.GetStringBytes()) == 0 {
			*value = *nullValue
		}
	case fastjson.TypeNumber:
		stringifyLargeInteger(value, arena, numberScratch)
	}
}

func stringifyLargeInteger(value *fastjson.Value, arena *fastjson.Arena, numberScratch *[]byte) {
	start := len(*numberScratch)
	*numberScratch = value.MarshalTo(*numberScratch)
	num := (*numberScratch)[start:]
	if shouldStringifyNumberBytes(num) {
		*value = *arena.NewStringBytes(num)
	}
	*numberScratch = (*numberScratch)[:start]
}

func shouldStringifyNumber(num string) bool {
	if len(num) == 0 {
		return false
	}
	return shouldStringifyNumberBytes([]byte(num))
}

func shouldStringifyNumberBytes(num []byte) bool {
	if len(num) == 0 {
		return false
	}

	for i := 0; i < len(num); i++ {
		c := num[i]
		if c == '.' || c == 'e' || c == 'E' {
			return false
		}
	}

	start := 0
	neg := num[0] == '-'
	if neg {
		start = 1
	}

	// Skip leading zeros
	for start < len(num) && num[start] == '0' {
		start++
	}

	digitLen := len(num) - start
	if digitLen == 0 {
		return false
	}

	const maxLen = 19
	if digitLen < maxLen {
		return false
	}
	if digitLen > maxLen {
		return true
	}

	digits := num[start:]
	if neg {
		const minInt64Abs = "9223372036854775808"
		return bytesGreaterThanString(digits, minInt64Abs)
	}
	const maxInt64 = "9223372036854775807"
	return bytesGreaterThanString(digits, maxInt64)
}

func bytesGreaterThanString(b []byte, s string) bool {
	for i := 0; i < len(b); i++ {
		if b[i] != s[i] {
			return b[i] > s[i]
		}
	}
	return false
}

func processLine(rawLine []byte, buf *bytes.Buffer) error {
	var p processor
	return p.processLine(rawLine, buf)
}

func (p *processor) processLine(rawLine []byte, buf *bytes.Buffer) error {
	value, err := p.parser.ParseBytes(rawLine)
	if err != nil {
		return fmt.Errorf("json parse error: %w", err)
	}

	p.arena.Reset()
	p.numberScratch = p.numberScratch[:0]

	removeEmptyStrings(value, &p.arena, &p.numberScratch)

	buf.Reset()
	buf.Grow(len(rawLine))
	output := value.MarshalTo(buf.AvailableBuffer())
	_, _ = buf.Write(output)

	p.arena.Reset()
	return nil
}

func main() {
	cpuProfile := flag.String("cpuprofile", "", "write CPU profile to file")
	flag.Parse()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cpuprofile create error: %v\n", err)
			os.Exit(1)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			fmt.Fprintf(os.Stderr, "cpuprofile start error: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = f.Close()
		}()
	}

	reader := bufio.NewReaderSize(os.Stdin, 4*1024*1024)
	writer := bufio.NewWriterSize(os.Stdout, 4*1024*1024)
	defer writer.Flush()
	buf := bytes.NewBuffer(make([]byte, 0, 64*1024))
	proc := processor{numberScratch: make([]byte, 0, 32)}

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "stdin read error: %v\n", err)
			return
		}

		if len(line) == 0 && err == io.EOF {
			return
		}

		hadNewline := false
		n := len(line)
		if n > 0 && line[n-1] == '\n' {
			hadNewline = true
			n--
		}
		if n > 0 && line[n-1] == '\r' {
			n--
		}
		line = line[:n]

		procErr := proc.processLine(line, buf)
		if procErr != nil {
			fmt.Fprintf(os.Stderr, "line processing error: %v\n", procErr)
			os.Exit(1)
		}

		_, _ = writer.Write(buf.Bytes())
		if hadNewline {
			_, _ = writer.WriteString("\n")
		}

		if err == io.EOF {
			return
		}
	}
}
