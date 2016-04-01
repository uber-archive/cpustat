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

package cpustat

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func tmpDir() (string, error) {
	tmpdir, err := ioutil.TempDir("", "spidlist_test.go")
	if err != nil {
		return tmpdir, err
	}
	procPath = tmpdir
	return tmpdir, nil
}

func mkfile(s string) {
	f, err := os.Create(s)
	if err != nil {
		panic(err)
	}
	f.Close()
}

func TestNormal(t *testing.T) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 200; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "foo"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "bar"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "baz"))
	pids := make(Pidlist, 0)
	GetPidList(&pids, 2048)
	if len(pids) != 200 {
		t.Error("pidlist should be 200 but is", len(pids))
	}
}

func TestTooLarge(t *testing.T) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 200; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "foo"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "bar"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "baz"))
	pids := make(Pidlist, 0)
	// surprising to me is that Readdirnames with a limit sometimes stops at limit - 1
	GetPidList(&pids, 20)
	if len(pids) != 19 && len(pids) != 20 {
		t.Error("pidlist should be 20 or less but is", len(pids))
	}
}

func TestEmpty(t *testing.T) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		t.Error(err)
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "one"))
	pids := make(Pidlist, 0)
	GetPidList(&pids, 2048)
	if len(pids) != 0 {
		t.Error("pidlist should be 0 but is", len(pids))
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "that"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "rug"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "really"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "tied"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "the"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "room"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "together"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "did"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "it"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "not"))
	GetPidList(&pids, 2048)
	if len(pids) != 0 {
		t.Error("pidlist should be 0 but is", len(pids))
	}
}

func TestGrowShrink(t *testing.T) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		t.Error(err)
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "foo"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "bar"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "baz"))

	for i := 0; i < 200; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}

	pids := make(Pidlist, 0)

	GetPidList(&pids, 2048)
	if len(pids) != 200 {
		t.Error("pidlist should be 200 but is", len(pids))
	}

	for i := 200; i < 400; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}

	GetPidList(&pids, 2048)
	if len(pids) != 400 {
		t.Error("pidlist should be 400 but is", len(pids))
	}

	for i := 100; i < 300; i++ {
		err := os.Remove(fmt.Sprintf("%s/%d", dirName, i))
		if err != nil {
			t.Error(err)
		}
	}

	GetPidList(&pids, 2048)
	if len(pids) != 200 {
		t.Error("pidlist should be 200 but is", len(pids))
	}
}

func BenchmarkSmallDir(b *testing.B) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < 200; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "foo"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "bar"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "baz"))
	pids := make(Pidlist, 0)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		GetPidList(&pids, 2048)
	}
}

func BenchmarkSmallDirNewSlice(b *testing.B) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < 200; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "foo"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "bar"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "baz"))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pids := make(Pidlist, 0)
		GetPidList(&pids, 2048)
	}
}

func BenchmarkLargeDir(b *testing.B) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < 2000; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "foo"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "bar"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "baz"))
	pids := make(Pidlist, 0)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		GetPidList(&pids, 2048)
	}
}

func BenchmarkLargeDirNewSlice(b *testing.B) {
	dirName, err := tmpDir()
	defer os.RemoveAll(dirName)
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < 2000; i++ {
		mkfile(fmt.Sprintf("%s/%d", dirName, i))
	}
	mkfile(fmt.Sprintf("%s/%s", dirName, "foo"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "bar"))
	mkfile(fmt.Sprintf("%s/%s", dirName, "baz"))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pids := make(Pidlist, 0)
		GetPidList(&pids, 2048)
	}
}
