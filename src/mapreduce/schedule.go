package mapreduce

import (
	"fmt"
	"sync"
)

// schedule starts and waits for all tasks in the given phase (Map or Reduce).
func (mr *Master) schedule(phase jobPhase) {
	var ntasks int
	var nios int // number of inputs (for reduce) or outputs (for map)
	switch phase {
	case mapPhase:
		ntasks = len(mr.files)
		nios = mr.nReduce
	case reducePhase:
		ntasks = mr.nReduce
		nios = len(mr.files)
	}

	fmt.Printf("Schedule: %v %v tasks (%d I/Os)\n", ntasks, phase, nios)

	// All ntasks tasks have to be scheduled on workers, and only once all of
	// them have been completed successfully should the function return.
	// Remember that workers may fail, and that any given worker may finish
	// multiple tasks.
	//
	// TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO
	//


	/*hzq
	for each task, get a worker from registerChannel and do the work by call rpc operation. if ok put the
	worker back into the registerChannel for other task.
	use waitgroup to wait all task finish
	*/
	var wg sync.WaitGroup
	for i := 0; i < ntasks; i++ {
		wg.Add(1)
		debug("hzq start i\n")
		go func(taskNum int, nios int, phase jobPhase, i int) {
			defer wg.Done()
			for {
				worker := <-mr.registerChannel
				doTaskArgs := DoTaskArgs{mr.jobName, mr.files[i], phase, i, nios}
				ok := call(worker, "Worker.DoTask", doTaskArgs, new(struct{}))
				/*hzq
				if break from else, the part 4 will failure because the failure task also
				break without retry.
				 */
				if ok {
					go func(){mr.registerChannel <- worker}()
					break
				} else {
					debug("worker: %s in jobname: %s error", worker, mr.jobName, i)
				}
			}
		}(ntasks, nios, phase, i)
	}
	wg.Wait()

	fmt.Printf("Schedule: %v phase done\n", phase)
}
