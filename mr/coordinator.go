package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"

	// go get github.com/pdfcpu/pdfcpu@latest
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
)

type MapTask struct {
	State  int // 0: idle, 1: Working, 2: Done
	Worker string
}

type ReduceTask struct {
	State  int
	Worker string
}
type Coordinator struct {
	// Your definitions here.
	mu               sync.Mutex
	DoneMap          bool
	DoneReduce       bool
	PdfPath          []string
	MapTaskBucket    [][]MapTask
	ReduceTaskBucket []ReduceTask
}

// Your code here -- RPC handlers for the worker to call.
func (c *Coordinator) Receiving(args *Args, reply *Reply) error {
	if args.WorkType == 0 {
		if !c.DoneMap {
			println("Map Serving..")
			reply.WorkType = 1 // map
			reply.FileName, reply.PageNum = c.TaskServing("temp")
			if reply.FileName == "" {
				reply.WorkType = 0
			}
		} else if !c.DoneReduce {
			println("Reduce Serving..")
			reply.WorkType = 2 //reduce
			reply.FileName = c.ReduceServing("temp")
			if reply.FileName == "" {
				reply.WorkType = 0
			}
		} else {
			reply.WorkType = 3 // TODO: Wait task, Not quit.
		}
	}
	return nil
}

func (c *Coordinator) DoneTask(args *Args, reply *Reply) error {
	if args.WorkType == 1 {
		i, _ := strconv.Atoi(args.FileName)
		j, _ := strconv.Atoi(args.PageNum)
		c.MapTaskBucket[i][j].State = 2
	}
	return nil
}

func (c *Coordinator) TaskServing(worker string) (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.MapTaskBucket {
		for j := range c.MapTaskBucket[i] {
			if c.MapTaskBucket[i][j].State == 0 {
				c.MapTaskBucket[i][j].State = 1
				// c.MapTaskBucket[i][j].Worker = worker
				return c.PdfPath[i], strconv.Itoa(j + 1)
			}
		}
	}

	c.DoneMap = true
	return "", ""
}

func (c *Coordinator) ReduceServing(worker string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.ReduceTaskBucket {
		if c.ReduceTaskBucket[i].State == 0 {
			c.ReduceTaskBucket[i].State = 1
			// c.ReduceTaskBucket[i].Worker = worker
			return c.PdfPath[i]
		}
	}

	c.DoneReduce = true
	return ""
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.

	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	c.PdfPath = append([]string(nil), files...)
	n := len(c.PdfPath)

	c.MapTaskBucket = make([][]MapTask, n)

	for i, p := range c.PdfPath {
		k, _ := pdfapi.PageCountFile(p)
		bucket := make([]MapTask, k)
		for page := 0; page < k; page++ {
			bucket[page] = MapTask{
				State:  0,
				Worker: "",
			}
		}
		c.MapTaskBucket[i] = bucket
	}

	// if nReduce <= 0 {
	// 	nReduce = 1
	// }
	nReduce = n
	c.ReduceTaskBucket = make([]ReduceTask, nReduce)
	for r := 0; r < nReduce; r++ {
		c.ReduceTaskBucket[r] = ReduceTask{
			State:  0,
			Worker: "",
		}
	}
	c.server()
	return &c
}
