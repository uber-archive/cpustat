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
	"strconv"
	"strings"
	"testing"
)

func TestStripSpecial(t *testing.T) {
	ret1 := strings.Map(StripSpecial, "1 2 3 4 5")
	if ret1 != "1 2 3 4 5" {
		t.Error("string was modified and shouldn't have been")
	}

	ret2 := strings.Map(StripSpecial, "aaa (bce) efg")
	if ret2 != "aaa bce efg" {
		t.Error("string was not stripped correctly")
	}

	ret3 := strings.Map(StripSpecial, "[aaa] (bce) efg")
	if ret3 != "aaa bce efg" {
		t.Error("string was not stripped correctly")
	}
}

var l1 = "0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48 49 50 51"
var l2 = "36101 ((sd-pam)) S 36099 36099 36099 0 -1 1077944640 27 0 0 0 0 0 0 0 20 0 1 0 319121869 56594432 984 18446744073709551615 1 1 0 0 0 0 0 4096 0 18446744073709551615 0 0 17 19 0 0 0 0 0 0 0 0 0 w x y z"
var l3 = "36099 (systemd) S 1 36099 36099 0 -1 4202752 895 22 0 0 1 1 0 0 20 0 1 0 319121869 28123136 964 18446744073709551615 1 1 0 0 0 0 671173123 4096 0 18446744073709551615 0 0 17 2 0 0 0 0 0 0 0 0 0 0 0 0 0"
var l4 = "17974 ([celeryd: celer) S 44582 44581 44581 0 -1 4202560 10130 0 0 0 59 13 0 0 20 0 3 0 317969348 965685248 19771 18446744073709551615 1 1 0 0 0 0 0 16781314 18949 18446744073709551615 0 0 17 2 0 0 0 0 0 0 0 0 0 0 0 0 0"
var l5 = "17974 ([celeryd:) celer) S 44582 44581 44581 0 -1 4202560 10130 0 0 0 59 13 0 0 20 0 3 0 317969348 965685248 19771 18446744073709551615 1 1 0 0 0 0 0 16781314 18949 18446744073709551615 0 0 17 2 0 0 0 0 0 0 0 0 0 0 0 0 0"
var l6 = "17974 ([celeryd: celer S 44582 44581 44581 0 -1 4202560 10130 0 0 0 59 13 0 0 20 0 3 0 317969348 965685248 19771 18446744073709551615 1 1 0 0 0 0 0 16781314 18949 18446744073709551615 0 0 17 2 0 0 0 0 0 0 0 0 0 0 0 0 0"

func TestProcPidStatSplit(t *testing.T) {
	parts1 := procPidStatSplit(l1)
	for i, part := range parts1 {
		val, err := strconv.ParseInt(part, 10, 32)
		if err != nil {
			t.Error("parsing number", part, err)
		}
		if i != int(val) {
			t.Error("field contents mismatch", i, part)
		}
	}
	if len(parts1) != 52 {
		t.Error("l1 split returned incorrect field count", 52)
	}

	parts2 := procPidStatSplit(l2)
	if len(parts2) != 52 {
		t.Error("l2 split returned incorrect field count", 52)
	}

	parts3 := procPidStatSplit(l3)
	if len(parts3) != 52 {
		t.Error("l3 split returned incorrect field count", 52)
	}

	parts4 := procPidStatSplit(l4)
	if len(parts4) != 52 {
		t.Error("l4 split returned incorrect field count", 52)
	}

	parts5 := procPidStatSplit(l5)
	if len(parts5) != 52 {
		t.Error("l5 split returned incorrect field count", 52)
	}

	parts6 := procPidStatSplit(l6)
	if len(parts6) != 52 {
		t.Error("l6 split returned incorrect field count", 52)
	}
	if parts6[0] != "17974" {
		t.Error("field 0 should be 17974 but is", parts6[0])
	}
	if parts6[1] != "" {
		t.Error("field 1 should be empty but is", parts6[1])
	}
	for i := 2; i <= 51; i++ {
		if parts6[i] != "" {
			t.Error("field ", i, "should be empty but is", parts6[i])
		}
	}
}
