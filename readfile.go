package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// note that this is not thread safe
var buf *bytes.Buffer

// ReadSmallFile is like os.ReadFile but skips the stat
func ReadSmallFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		f.Close()
		return nil, err
	}

	if buf == nil {
		buf = bytes.NewBuffer(make([]byte, 0, 8192))
	} else {
		buf.Reset()
	}
	_, err = buf.ReadFrom(f)
	f.Close()
	return buf.Bytes(), err
}

func readFileLines(filename string) ([]string, error) {
	file, err := ReadSmallFile(filename)
	if err != nil {
		return nil, err
	}

	fileStr := strings.TrimSpace(string(file))
	return strings.Split(fileStr, "\n"), nil
}

func readFloat(str string) float64 {
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		log.Fatal(err)

	}
	return val
}

func readUInt(str string) uint64 {
	val, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return val
}

func readInt(str string) int64 {
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	return val
}

func trim(num float64, max int) string {
	if num == 0.0 {
		return "0"
	}
	var str string
	if num >= 1000.0 {
		str = fmt.Sprintf("%d", int(num+0.5))
	} else {
		str = fmt.Sprintf("%.1f", num)
	}
	if len(str) > max {
		if str[max-1] == 46 { // ASCII .
			return str[:max-1]
		}
		return str[:max]
	}
	return str
}

func trunc(str string, length int) string {
	if len(str) <= length {
		return str
	}
	return str[:length]
}
