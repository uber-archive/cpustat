package main

import (
	"fmt"
	"os"

	lib "github.com/uber-common/cpustat/lib"
)

func main() {
	conn := lib.NLInit()
	fmt.Println(conn)

	ts, str, err := lib.TaskStatsLookupPid(conn, os.Getpid())
	fmt.Println("ts: ", ts)
	fmt.Println("str: ", str)
	fmt.Println("err: ", err)
}
