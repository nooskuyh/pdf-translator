package main

// a word-count application "plugin" for MapReduce.
//
// go build -buildmode=plugin wc.go
// go get github.com/openai/openai-go/v3

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mr "pdfmr/mr"

	"github.com/jung-kurt/gofpdf"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

var (
	clientOnce sync.Once
	client     openai.Client
	clientErr  error
)

func getClient() (*openai.Client, error) {
	clientOnce.Do(func() {
		if os.Getenv("OPENAI_API_KEY") == "" {
			clientErr = fmt.Errorf("OPENAI_API_KEY is not set in this worker process")
			return
		}
		client = openai.NewClient()
	})
	if clientErr != nil {
		return nil, clientErr
	}
	return &client, nil
}

func Map(pagenumber string, contents string) mr.PageKV {
	c, err := getClient()
	if err != nil {
		return mr.PageKV{Key: pagenumber, Value: "ERROR: " + err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("API Call (start)")

	resp, err := c.Responses.New(ctx, responses.ResponseNewParams{
		Model:        "o4-mini",
		Instructions: openai.String("이 내용을 자연스러운 한글로 번역, 깔끔하게 하나의 Pdf 페이지에 들어갈 수 있는 문자열로"),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(contents),
		},
	})

	if err != nil {
		fmt.Printf("API Call (error): %v\n", err)
		return mr.PageKV{Key: pagenumber, Value: "ERROR: " + err.Error()}
	}
	if resp == nil {
		return mr.PageKV{Key: pagenumber, Value: "ERROR: nil response without error"}
	}

	answer := strings.TrimSpace(resp.OutputText())
	if answer == "" {
		answer = "ERROR: empty output text"
	}

	fmt.Println("API Call (done)")
	return mr.PageKV{Key: pagenumber, Value: answer}
}

func Reduce(key string, values []string) string {
	CombineToPDF(key)
	return ""
}

func CombineToPDF(filename string) error {
	type item struct {
		i    int
		path string
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		return fmt.Errorf("readdir %q: %w", cwd, err)
	}

	pat := regexp.MustCompile(`^mr-(\d+)-(.+)$`)

	var items []item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		m := pat.FindStringSubmatch(name)
		if m == nil || m[2] != filename {
			continue
		}
		i, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		items = append(items, item{i: i, path: filepath.Join(cwd, name)})
	}

	if len(items) == 0 {
		return fmt.Errorf("no inputs found in %q matching mr-{i}-%s", cwd, filename)
	}

	sort.SliceStable(items, func(a, b int) bool { return items[a].i < items[b].i })

	outPDF := "traslated_" + filename
	const family = "NotoSansKR"

	regularPath := "../fonts/NotoSansKR-Regular.ttf"

	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(15, 15, 15)
	p.SetAutoPageBreak(true, 15)

	p.AddUTF8Font(family, "", regularPath)
	p.AddUTF8Font(family, "B", regularPath)

	if err := p.Error(); err != nil {
		return fmt.Errorf("pdf font setup: %w", err)
	}

	p.SetFont(family, "", 11)

	for _, it := range items {
		p.AddPage()

		p.SetFont(family, "B", 14)
		p.MultiCell(0, 7, fmt.Sprintf("Part %d: %s", it.i, filepath.Base(it.path)), "", "L", false)
		p.Ln(2)

		p.SetFont(family, "", 11)

		f, err := os.Open(it.path)
		if err != nil {
			return fmt.Errorf("open %q: %w", it.path, err)
		}

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 10*1024*1024)

		for sc.Scan() {
			p.MultiCell(0, 5, sc.Text(), "", "L", false)
		}

		if err := sc.Err(); err != nil {
			_ = f.Close()
			return fmt.Errorf("scan %q: %w", it.path, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close %q: %w", it.path, err)
		}
	}

	if err := p.OutputFileAndClose(outPDF); err != nil {
		return fmt.Errorf("write output %q: %w", outPDF, err)
	}
	return nil
}
