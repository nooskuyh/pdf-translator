package mr

import (
	"fmt"
	"hash/fnv"
	"log"
	"net/rpc"
	"os"
	"strconv"

	pdf "github.com/ledongthuc/pdf"
)

type PageKV struct {
	Key   string
	Value string
}

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value PageKV
}

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

func ExtractPDFPageText(pdfPath string, pageNum int) (string, error) {
	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if pageNum < 1 || pageNum > r.NumPage() {
		return "", fmt.Errorf("page out of range: %d (1..%d)", pageNum, r.NumPage())
	}

	p := r.Page(pageNum)
	if p.V.IsNull() {
		return "", fmt.Errorf("page %d is null", pageNum)
	}

	text, err := p.GetPlainText(nil) // pass nil if you don't want to manage a font cache
	if err != nil {
		return "", err
	}
	return text, nil
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) PageKV,
	reducef func(string, []string) string) {
	for {
		args := Args{0, "", ""}
		reply := Reply{}
		MyCall(&args, &reply)
		if reply.WorkType == 1 {
			pageNum, err := strconv.Atoi(reply.PageNum)
			if err != nil {
				log.Printf("invalid page number %q: %v", reply.FileName, err)
				continue
			}
			content, err := ExtractPDFPageText(reply.FileName, pageNum)
			if err != nil {
				log.Printf("pdf extract failed (pdf=%q page=%d): %v", reply.FileName, pageNum, err)
				continue
			}
			answer := mapf(reply.PageNum, content).Value
			filePath := fmt.Sprintf("mr-%s-%s", reply.PageNum, reply.FileName)

			f, err := os.Create(filePath)
			if err != nil {
				log.Fatalf("create %q: %v", filePath, err)
			}
			defer f.Close()

			if _, err := f.WriteString(answer); err != nil {
				log.Fatalf("write to %q: %v", filePath, err)
			}
			if len(answer) == 0 || answer[len(answer)-1] != '\n' {
				if _, err := f.WriteString("\n"); err != nil {
					log.Fatalf("write newline to %q: %v", filePath, err)
				}
			}
			args.WorkType = 1
			args.FileName = reply.FileName
			args.PageNum = reply.PageNum
			DoneCall(&args, &reply)
		}

		if reply.WorkType == 2 {
			fmt.Println("Get Reduce----")
			reducef(reply.FileName, []string{})
			args.WorkType = 2
			args.FileName = reply.FileName
			DoneCall(&args, &reply)
		}

		if reply.WorkType == 3 {
			break
		}
	}

	fmt.Println("GoodBye!")
}

func MyCall(args *Args, reply *Reply) {
	fmt.Printf("Calling...(Task Request): %v\n", args.WorkType)
	ok := call("Coordinator.Receiving", &args, &reply)
	if ok {
		if reply.FileName == "" {
			// fmt.Print("Task Waiting...")
			return
		}
		fmt.Printf("- worktype: %v\n", reply.WorkType)
		fmt.Printf("- filename: %v\n", reply.FileName)
		fmt.Printf("- PageNum: %v\n", reply.PageNum)
	} else {
		fmt.Printf("call failed!\n")
	}
}

func DoneCall(args *Args, reply *Reply) {
	fmt.Printf("Calling...(Task Done): %s, %s, %v\n", args.FileName, args.PageNum, args.WorkType)
	ok := call("Coordinator.Receiving", &args, &reply)
	if ok {
		fmt.Printf("- worktype: %v\n", reply.WorkType)
		fmt.Printf("- filename: %v\n", reply.FileName)
		fmt.Printf("- PageNum: %v\n", reply.PageNum)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	//sockname := coordinatorSock()

	addr := os.Getenv("MR_COORD_ENDPOINT")
	if addr == "" {
		addr = "mr-master:1234"
	}

	c, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		log.Printf("dialing: %v", err)
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err != nil {
		log.Printf("rpc call %s error: %v", rpcname, err)
		return false
	}

	return true
}
