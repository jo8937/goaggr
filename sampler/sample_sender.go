package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"bytes"
	"compress/gzip"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/logger"
)

// 샘플링 옵션
type SamplingConfig struct {
	url        string
	datafile   string
	total      int
	concurrent int
}

var (
	wg               sync.WaitGroup
	config           *SamplingConfig
	secondaryLogFile *os.File
	secondaryLogger  *logger.Logger
)

func InitFileLogger(logPath string) {
	secondaryLogFile, secondaryLogger = createFileLogger(logPath)
}

func CloseFileLogger() {
	defer secondaryLogFile.Close()
	defer secondaryLogger.Close()
}

func createFileLogger(logPath string) (*os.File, *logger.Logger) {
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		logger.Fatalf("Failed to open log file: %v", err)
	}
	lg := logger.Init("FluentdFails", false, false, lf)

	return lf, lg
}

func createConfig() *SamplingConfig {
	c := new(SamplingConfig)
	flag.StringVar(&c.url, "url", "http://localhost:8080", "TCP address to listen to")
	flag.StringVar(&c.datafile, "datafile", "sample.json", "sample post data file script path")
	flag.IntVar(&c.total, "total", 100, "total send data")
	flag.IntVar(&c.concurrent, "concurrent", 8, "concurrent count")
	flag.Parse()
	return c
}
func getDataFileRealPath(datafile string) string {
	realpath, err := filepath.Abs(datafile)
	if err != nil {
		log.Fatal(err)
	}
	return realpath
}

func initAnsSendJSONSample() {
	config := createConfig()
	SendJSONSample(config)
}

// send datafile to POST cnt times to url
func SendJSONSample(config *SamplingConfig) {

	jsonByte, err := ioutil.ReadFile(getDataFileRealPath(config.datafile))
	if err != nil {
		panic(err)
	}

	//reqBodyBuf := bytes.NewBufferString(str)
	var gzipBuf bytes.Buffer
	g := gzip.NewWriter(&gzipBuf)

	if _, err := g.Write(jsonByte); err != nil {
		log.Print(err)
		return
	}
	if err := g.Close(); err != nil {
		log.Print(err)
		return
	}
	totalCnt := 0
	for i, loopMax := 1, 1+int(config.total/config.concurrent); i <= loopMax; i++ {

		concurrentMax := config.concurrent

		if concurrentMax*i > config.total {
			concurrentMax = config.total % concurrentMax
		}
		for n := 0; n < concurrentMax; n++ {
			wg.Add(1)
			go func(gzipBuf bytes.Buffer) {
				req, err := http.NewRequest("POST", config.url, &gzipBuf)
				if err != nil {
					panic(err)
				}
				req.Header.Set("X-Custom-Header", "myvalue")
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Content-Encoding", "gzip")

				client := &http.Client{}
				resp, err := client.Do(req)
				//client = createFluentdClient()
				//defer closeFluentd()
				// TOSO send http
				if err != nil {
					panic(err)
				}
				log.Print(resp.StatusCode)

				respBody, err := ioutil.ReadAll(resp.Body)
				if err == nil {
					resText := string(respBody)
					log.Printf(resText)
				}
				wg.Done()
				totalCnt++
			}(gzipBuf)
		}
		wg.Wait()
	}

	log.Printf("send %d data finish", totalCnt)
}

func main() {
	initAnsSendJSONSample()
}
