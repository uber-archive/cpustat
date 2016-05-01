package cpustat

import (
	"fmt"
	"log"
	"os/user"
	"regexp"
	"sort"
	"strconv"
)

var splitter = regexp.MustCompile("[, ] *")

// ParseUsrList take a string of Unix usernames and converts it into a slice of int userids
func ParseUserList(s string) ([]int, error) {
	parts := splitter.Split(s, -1)
	ret := make([]int, len(parts))
	for pos, part := range parts {
		userEnt, err := user.Lookup(part)
		if err != nil {
			return nil, err
		}

		uid, err := strconv.Atoi(userEnt.Uid)
		if err != nil {
			return nil, err
		}
		ret[pos] = uid
	}
	return ret, nil
}

// ParsePidList take a string of process ids and converts it into a list of int pids
func ParsePidList(s string) ([]int, error) {
	parts := splitter.Split(s, -1)
	ret := make([]int, len(parts))
	for pos, part := range parts {
		if len(part) == 0 {
			continue
		}
		num, err := strconv.Atoi(part)
		if err != nil {
			panic(err)
		}
		ret[pos] = num
	}
	return ret, nil
}

type Filters struct {
	User    []int
	UserStr []string
	Pid     []int
	PidStr  []string
}

func (f Filters) PidMatch(pid int) bool {
	if len(f.Pid) == 0 {
		return true
	}

	pos := sort.SearchInts(f.Pid, pid)
	if pos == len(f.Pid) {
		return false
	}
	if f.Pid[pos] == pid {
		return true
	}
	return false
}

func (f Filters) UserMatch(user int) bool {
	if len(f.User) == 0 {
		return true
	}

	pos := sort.SearchInts(f.User, user)
	if pos == len(f.User) {
		return false
	}
	if f.User[pos] == user {
		return true
	}
	return false
}

func FiltersInit(user, pid string) Filters {
	ret := Filters{}

	var err error
	if user != "" {
		ret.User, err = ParseUserList(user)
		if err != nil {
			log.Fatal(err)
		}
		sort.Ints(ret.User)
		ret.UserStr = make([]string, len(ret.User))
		for i := range ret.User {
			ret.UserStr[i] = fmt.Sprint(ret.User[i])
		}
	}

	if pid != "" {
		ret.Pid, err = ParsePidList(pid)
		if err != nil {
			log.Fatal(err)
		}
		sort.Ints(ret.Pid)
		ret.PidStr = make([]string, len(ret.Pid))
		for i := range ret.Pid {
			ret.PidStr[i] = fmt.Sprint(ret.Pid[i])
		}
	}

	return ret
}
