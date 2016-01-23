package main

import (
	"strconv"
	"testing"
)

func TestProcPidStatSplit(t *testing.T) {
	l1 := "0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48 49 50 51"
	l2 := "36101 ((sd-pam)) S 36099 36099 36099 0 -1 1077944640 27 0 0 0 0 0 0 0 20 0 1 0 319121869 56594432 984 18446744073709551615 1 1 0 0 0 0 0 4096 0 18446744073709551615 0 0 17 19 0 0 0 0 0 0 0 0 0 w x y z"
	l3 := "36099 (systemd) S 1 36099 36099 0 -1 4202752 895 22 0 0 1 1 0 0 20 0 1 0 319121869 28123136 964 18446744073709551615 1 1 0 0 0 0 671173123 4096 0 18446744073709551615 0 0 17 2 0 0 0 0 0 0 0 0 0 0 0 0 0"
	l4 := "17974 ([celeryd: celer) S 44582 44581 44581 0 -1 4202560 10130 0 0 0 59 13 0 0 20 0 3 0 317969348 965685248 19771 18446744073709551615 1 1 0 0 0 0 0 16781314 18949 18446744073709551615 0 0 17 2 0 0 0 0 0 0 0 0 0 0 0 0 0"

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

}
