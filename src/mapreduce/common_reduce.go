package mapreduce

import (
	"encoding/json"
	"os"
	"log"
	"sort"
)

// doReduce does the job of a reduce worker: it reads the intermediate
// key/value pairs (produced by the map phase) for this task, sorts the
// intermediate key/value pairs by key, calls the user-defined reduce function
// (reduceF) for each key, and writes the output to disk.
func doReduce(
	jobName string, // the name of the whole MapReduce job
	reduceTaskNumber int, // which reduce task this is
	nMap int, // the number of map tasks that were run ("M" in the paper)
	reduceF func(key string, values []string) string,
) {
	// TODO:
	// You will need to write this function.
	// You can find the intermediate file for this reduce task from map task number
	// m using reduceName(jobName, m, reduceTaskNumber).
	// Remember
	//
	// You should write the reduced output in as JSON encoded KeyValue
	// objects to a file named mergeName(jobName, reduceTaskNumber). We require
	// you to use JSON here because that is what the merger than combines the
	// output from all the reduce tasks expects. There is nothing "special" about
	// JSON -- it is just the marshalling format we chose to use. It will look
	// something like this:
	//
	// enc := json.NewEncoder(mergeFile)
	// for key in ... {
	// 	enc.Encode(KeyValue{key, reduceF(...)})
	// }
	// file.Close()

	keyValues := make(map[string][]string)
	//hzq: 1 read all map result
	for i := 0 ; i< nMap ; i++ {
		mapResultFileName := reduceName(jobName, i, reduceTaskNumber)
		mapResultFile, err := os.Open(mapResultFileName)
		if err!= nil {
			log.Fatal ("open file error", err)
		}
		dec := json.NewDecoder(mapResultFile)
		// for without param means infinite loop
		for {
			var kv KeyValue
			err := dec.Decode(&kv)
			if err != nil {
				break
			}
			_, ok := keyValues[kv.Key]
			if !ok {
				keyValues[kv.Key] = make([]string,0)
			}
			keyValues[kv.Key] = append(keyValues[kv.Key], kv.Value)
		}
	}

	//hzq 2 sort keys
	var keys []string
	for k,_ := range keyValues {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	mergeFileName := mergeName(jobName, reduceTaskNumber)
	file, err := os.Create(mergeFileName)
	if err!= nil {
		log.Fatal("create file err", err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)

	for _, k := range keys {
		res := reduceF(k, keyValues[k])
		enc.Encode(&KeyValue{k, res})
	}

}
