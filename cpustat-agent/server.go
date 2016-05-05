// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// This program gathers the metrics from the system and writes them to lmdb where
// various other programs can come and get them

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/uber-common/cpustat/lib"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/raw"
	"golang.org/x/net/context"
)

type rawHandler struct {
	memdb   *MemDB
	infoMap cpustat.ProcInfoMap
}

func (r rawHandler) Handle(ctx context.Context, args *raw.Args) (*raw.Res, error) {
	switch args.Method {
	case "readSamples":
		return &raw.Res{
			Arg2: []byte{},
			Arg3: gobEncodeSamples(args.Arg3, r),
		}, nil
	case "readSys":
		return &raw.Res{
			Arg2: []byte{},
			Arg3: gobEncodeSys(args.Arg3, r),
		}, nil
	}
	return nil, fmt.Errorf("unhandled: (%s)", args.Method)
}

func gobEncodeSys(countBytes []byte, r rawHandler) []byte {
	count := binary.LittleEndian.Uint32(countBytes)

	samples := r.memdb.ReadSamples(count)
	var valBuf bytes.Buffer
	enc := gob.NewEncoder(&valBuf)

	if err := enc.Encode(time.Now()); err != nil {
		panic(err)
	}

	sendCount := uint32(len(samples))

	if err := enc.Encode(sendCount); err != nil {
		panic(err)
	}

	for _, sample := range samples {
		if err := enc.Encode(sample.Sys); err != nil {
			panic(err)
		}
	}
	return valBuf.Bytes()
}

func gobEncodeSamples(countBytes []byte, r rawHandler) []byte {
	count := binary.LittleEndian.Uint32(countBytes)

	samples := r.memdb.ReadSamples(count)
	var valBuf bytes.Buffer
	enc := gob.NewEncoder(&valBuf)

	if err := enc.Encode(time.Now()); err != nil {
		panic(err)
	}

	infolock.Lock()
	if err := enc.Encode(r.infoMap); err != nil {
		panic(err)
	}
	infolock.Unlock()

	if err := enc.Encode(intervalms); err != nil {
		panic(err)
	}

	sendCount := uint32(len(samples))

	if err := enc.Encode(sendCount); err != nil {
		panic(err)
	}

	for _, sample := range samples {
		if err := enc.Encode(sample.Proc.Samples[0:sample.Proc.Len]); err != nil {
			panic(err)
		}
		if err := enc.Encode(sample.Sys); err != nil {
			panic(err)
		}
	}
	return valBuf.Bytes()
}

func (rawHandler) OnError(ctx context.Context, err error) {
	log.Fatalf("OnError: %v", err)
}

func runServer(memdb *MemDB, infoMap cpustat.ProcInfoMap) {
	ch, err := tchannel.NewChannel("cpustat", nil)
	if err != nil {
		log.Fatalf("NewChannel failed: %v", err)
	}

	handler := raw.Wrap(rawHandler{memdb, infoMap})
	ch.Register(handler, "readSamples")
	ch.Register(handler, "readSys")
	ch.Register(handler, "status")

	hostPort := fmt.Sprintf("%s:%v", "127.0.0.1", 1971)
	if err := ch.ListenAndServe(hostPort); err != nil {
		log.Fatalf("ListenAndServe failed: %v", err)
	}

	fmt.Println("listening on", ch.PeerInfo().HostPort)
	select {}
}
