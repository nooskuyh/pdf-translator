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

// / Your code here -- RPC handlers for the worker to call.
func (c *Coordinator) Receiving(args *Args, reply *Reply) error {
	if !c.DoneMap {
		c.MapChecking()
	}
	if !c.DoneReduce {
		c.ReduceChecking()
	}
	if args.WorkType == 0 {
		if !c.DoneMap {
			reply.WorkType = 1 // map
			reply.FileName, reply.PageNum = c.TaskServing("temp")
			if reply.FileName == "" {
				reply.WorkType = 0
			}
		} else if !c.DoneReduce {
			reply.WorkType = 2 //reduce
			reply.FileName = c.ReduceServing("temp")
			if reply.FileName == "" {
				reply.WorkType = 0
			}
		} else {
			reply.WorkType = 3

		}
	} else if args.WorkType == 1 {
		log.Printf("%s %s", args.FileName, args.PageNum)
		i := c.GetPdfIndex(args.FileName)
		j, _ := strconv.Atoi(args.PageNum)
		log.Printf("File: %i, Page: %j: Map Done", i, j-1)
		c.MapTaskBucket[i][j-1].State = 2
	} else if args.WorkType == 2 {
		i := c.GetPdfIndex(args.FileName)
		log.Printf("File: %i, Reduce Done", i)
		c.ReduceTaskBucket[i].State = 2
	}
	return nil
}

func (c *Coordinator) TaskServing(worker string) (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.MapTaskBucket {
		for j := range c.MapTaskBucket[i] {
			if c.MapTaskBucket[i][j].State == 0 {
				println("Map Serving..:", i, j)
				c.MapTaskBucket[i][j].State = 1
				// c.MapTaskBucket[i][j].Worker = worker
				return c.PdfPath[i], strconv.Itoa(j + 1)
			}
		}
	}
	return "", ""
}

func (c *Coordinator) MapChecking() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.MapTaskBucket {
		for j := range c.MapTaskBucket[i] {
			if c.MapTaskBucket[i][j].State != 2 {
				return
			}
		}
	}
	println("Map tasks has Done.")
	c.DoneMap = true
}

func (c *Coordinator) ReduceChecking() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.ReduceTaskBucket {
		if c.ReduceTaskBucket[i].State != 2 {
			return
		}
	}
	println("Reduce tasks has Done.")
	c.DoneReduce = true
}

func (c *Coordinator) ReduceServing(worker string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.ReduceTaskBucket {
		if c.ReduceTaskBucket[i].State == 0 {
			println("Reduce Serving..:", i)
			c.ReduceTaskBucket[i].State = 1
			// c.ReduceTaskBucket[i].Worker = worker
			return c.PdfPath[i]
		}
	}
	return ""
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")

	//sockname := coordinatorSock()
	//os.Remove(sockname)

	addr := os.Getenv("MR_LISTEN_ADDR")
	if addr == "" {
		addr = "0.0.0.0:1234"
	}

	l, e := net.Listen("tcp", addr)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	//	ret := false
	// Your code here.
	return c.DoneReduce
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

func (c *Coordinator) GetPdfIndex(pdfPath string) int {
	for i, v := range c.PdfPath {
		if v == pdfPath {
			return i
		}
	}
	return -1
}
