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
	"bytes"
	"fmt"
	"testing"
)

func TestFiltersEmpty(t *testing.T) {
	f := FiltersInit("", "")
	if f.PidMatch(0) == false {
		t.Error("PidMatch(0) should be false")
	}

	if f.UserMatch(0) == false {
		t.Error("UserMatch(0) should be false")
	}
}

func TestFiltersPids(t *testing.T) {
	f1 := FiltersInit("", "123")
	if f1.PidMatch(0) == true {
		t.Error("PidMatch(0) should be false")
	}
	if f1.PidMatch(123) == false {
		t.Error("PidMatch(123) should be true")
	}

	f2 := FiltersInit("", "123,456")
	if f2.PidMatch(0) == true {
		t.Error("PidMatch(0) should be false")
	}
	if f2.PidMatch(123) == false {
		t.Error("PidMatch(123) should be true")
	}
	if f2.PidMatch(456) == false {
		t.Error("PidMatch(456) should be true")
	}

	f3 := FiltersInit("", "123, 456")
	if f3.PidMatch(0) == true {
		t.Error("PidMatch(0) should be false")
	}
	if f3.PidMatch(123) == false {
		t.Error("PidMatch(123) should be true")
	}
	if f3.PidMatch(456) == false {
		t.Error("PidMatch(456) should be true")
	}

	f4 := FiltersInit("", "123 456")
	if f4.PidMatch(0) == true {
		t.Error("PidMatch(0) should be false")
	}
	if f4.PidMatch(123) == false {
		t.Error("PidMatch(123) should be true")
	}
	if f4.PidMatch(456) == false {
		t.Error("PidMatch(456) should be true")
	}

	f5 := FiltersInit("", "123,456, 768")
	if f5.PidMatch(0) == true {
		t.Error("PidMatch(0) should be false")
	}
	if f5.PidMatch(123) == false {
		t.Error("PidMatch(123) should be true")
	}
	if f5.PidMatch(456) == false {
		t.Error("PidMatch(456) should be true")
	}
	if f5.PidMatch(768) == false {
		t.Error("PidMatch(768) should be true")
	}

	f6 := FiltersInit("", "123 456  768")
	if f6.PidMatch(0) == true {
		t.Error("PidMatch(0) should be false")
	}
	if f6.PidMatch(456) == false {
		t.Error("PidMatch(456) should be true")
	}
	if f6.PidMatch(768) == false {
		t.Error("PidMatch(768) should be true")
	}
}

func TestFiltersUsers(t *testing.T) {
	f1 := FiltersInit("root", "")
	if f1.UserMatch(-1) == true {
		t.Error("UserMatch(-1) should be false")
	}
	if f1.UserMatch(0) == false {
		t.Error("UserMatch(0) should be true")
	}

	f2 := FiltersInit("root,daemon", "")
	if f2.UserMatch(-1) == true {
		t.Error("UserMatch(-1) should be false")
	}
	if f2.UserMatch(0) == false {
		t.Error("UserMatch(0) should be true")
	}
	if f2.UserMatch(1) == false {
		t.Error("UserMatch(1) should be true")
	}

	f3 := FiltersInit("root, daemon", "")
	if f3.UserMatch(-1) == true {
		t.Error("UserMatch(-1) should be false")
	}
	if f3.UserMatch(0) == false {
		t.Error("UserMatch(0) should be true")
	}
	if f3.UserMatch(1) == false {
		t.Error("UserMatch(1) should be true")
	}

	f4 := FiltersInit("root daemon", "")
	if f4.UserMatch(-1) == true {
		t.Error("UserMatch(-1) should be false")
	}
	if f4.UserMatch(0) == false {
		t.Error("UserMatch(0) should be true")
	}
	if f4.UserMatch(1) == false {
		t.Error("UserMatch(1) should be true")
	}

	f5 := FiltersInit("root, daemon,  bin", "")
	if f5.UserMatch(-1) == true {
		t.Error("UserMatch(-1) should be false")
	}
	if f5.UserMatch(0) == false {
		t.Error("UserMatch(0) should be true")
	}
	if f5.UserMatch(1) == false {
		t.Error("UserMatch(1) should be true")
	}
	if f5.UserMatch(2) == false {
		t.Error("UserMatch(2) should be true")
	}

	f6 := FiltersInit("root daemon  bin", "")
	if f6.UserMatch(-1) == true {
		t.Error("UserMatch(-1) should be false")
	}
	if f6.UserMatch(0) == false {
		t.Error("UserMatch(0) should be true")
	}
	if f6.UserMatch(1) == false {
		t.Error("UserMatch(1) should be true")
	}
	if f6.UserMatch(2) == false {
		t.Error("UserMatch(2) should be true")
	}
}

func BenchmarkFiltersPidNotFoundSmall(b *testing.B) {
	f6 := FiltersInit("", "123")
	for i := 0; i < b.N; i++ {
		f6.UserMatch(0)
	}
}

func BenchmarkFiltersPidNotFoundLarge(b *testing.B) {
	list := bytes.NewBuffer(nil)
	for i := 0; i < 1000; i++ {
		list.WriteString(fmt.Sprint(i))
		list.WriteString(" ")
	}
	f6 := FiltersInit("", string(list.Bytes()))
	for i := 0; i < b.N; i++ {
		f6.UserMatch(0)
	}
}

func BenchmarkFiltersPidFoundSmall(b *testing.B) {
	f6 := FiltersInit("", "123")
	for i := 0; i < b.N; i++ {
		f6.UserMatch(123)
	}
}

func BenchmarkFiltersPidFoundLarge(b *testing.B) {
	list := bytes.NewBuffer(nil)
	for i := 0; i < 1000; i++ {
		list.WriteString(fmt.Sprint(i))
		list.WriteString(" ")
	}
	f6 := FiltersInit("", string(list.Bytes()))
	for i := 0; i < b.N; i++ {
		f6.UserMatch(200)
	}
}
