package cpustat

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"time"

	"github.com/bmatsuo/lmdb-go/lmdb"
)

type DbHandle struct {
	env *lmdb.Env
	dbi lmdb.DBI
}

// flags for DbInit
const (
	Reader = 1
	Writer = 2
)

// DbInit opens the lmdb database at path as either a writer or a reader
func DbInit(path string, mode int) *DbHandle {
	if mode != Reader && mode != Writer {
		panic("DbInit bad mode specified")
	}

	if mode == Reader {
		info, err := os.Stat(path)
		if err != nil {
			panic(fmt.Sprintf("error path %s not found: %s", path, err))
		}
		if !info.IsDir() {
			panic(fmt.Sprintf("db path %s not a directory", path))
		}
	} else {
		info, err := os.Stat(path)
		if err != nil {
			err := os.Mkdir(path, 0755)
			if err != nil {
				panic(fmt.Sprintf("error mkdir %s for db: %s", path, err))
			}
		}
		info, err = os.Stat(path)
		if err != nil {
			panic(fmt.Sprintf("error after mkdir %s for db: %s", path, err))
		}
		if !info.IsDir() {
			panic(fmt.Sprintf("db path %s already has non-directory", path))
		}
	}

	handle := DbHandle{}

	dbEnv, err := lmdb.NewEnv()
	if err != nil {
		panic(fmt.Sprint("NewEnv ", err))
	}

	err = dbEnv.SetMaxDBs(1)
	if err != nil {
		panic(fmt.Sprint("SetMaxDBs ", err))
	}
	err = dbEnv.SetMapSize(10 * 1024 * 1024 * 1024)
	if err != nil {
		panic(fmt.Sprint("SetMapSize ", err))
	}
	if mode == Reader {
		fmt.Println("Opening Readonly")
		err = dbEnv.Open(path, lmdb.Readonly, 0644)
	} else {
		err = dbEnv.Open(path, 0, 0644)
	}
	if err != nil {
		panic(fmt.Sprint("Open ", err))
	}

	handle.env = dbEnv

	info, err := dbEnv.Info()
	fmt.Println("dbInit ", mode, " info ", info, err)

	// A DBI handle can be used as long as the enviroment is mapped.
	if mode == Reader {
		err = dbEnv.View(func(txn *lmdb.Txn) (err error) {
			dbDBI, _ := txn.OpenDBI("cpustat", lmdb.Readonly)
			handle.dbi = dbDBI
			return err
		})
	} else {
		err = dbEnv.Update(func(txn *lmdb.Txn) (err error) {
			dbDBI, _ := txn.OpenDBI("cpustat", lmdb.Create)
			handle.dbi = dbDBI
			return err
		})
	}
	if err != nil {
		panic(fmt.Sprint("Create ", err))
	}

	return &handle
}

// format is:
//   key: time.Time
//   Value: procStatsMap, taskStatsMap, systemStats

// WriteSample takes the current update and writes it to the db as a gob stream
func (handle *DbHandle) WriteSample(proc *ProcStatsMap, task *TaskStatsMap, sys *SystemStats) {
	err := handle.env.Update(func(txn *lmdb.Txn) (err error) {
		key, err := time.Now().MarshalBinary()
		if err != nil {
			return err
		}

		var valBuf bytes.Buffer
		enc := gob.NewEncoder(&valBuf)
		err = enc.Encode(proc)
		if err != nil {
			panic(err)
		}
		err = enc.Encode(task)
		if err != nil {
			panic(err)
		}
		err = enc.Encode(sys)
		if err != nil {
			panic(err)
		}
		val := valBuf.Bytes()

		err = txn.Put(handle.dbi, key, val, 0)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		panic(fmt.Sprint("Update ", err))
	}
}

type SampleBatch struct {
	proc *ProcStats
	task *TaskStats
	sys  *SystemStats
}

// ReadSample fetches the nearest sample to start
func (handle *DbHandle) ReadSample(start time.Time) []SampleBatch {
	var batches []SampleBatch
	err := handle.env.View(func(txn *lmdb.Txn) (err error) {
		cur, err := txn.OpenCursor(handle.dbi)
		if err != nil {
			return err
		}

		for {
			k, v, err := cur.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				fmt.Println("end of db")
				return nil
			} else if err != nil {
				return err
			}

			t := time.Time{}
			t.UnmarshalBinary(k)

			buf := bytes.NewBuffer(v)
			dec := gob.NewDecoder(buf)
			var proc ProcStats
			err = dec.Decode(&proc)
			var task TaskStats
			err = dec.Decode(&task)
			var sys SystemStats
			err = dec.Decode(&sys)

			batch := SampleBatch{&proc, &task, &sys}
			batches = append(batches, batch)
		}
	})
	if err != nil {
		panic(err)
	}
	return batches
}

func (handle *DbHandle) Close() {
	handle.env.Close()
}
