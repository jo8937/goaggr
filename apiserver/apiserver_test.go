package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go
// test
import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"
)

var wg sync.WaitGroup

func GetSampleLogJSON() string {
	return `{
        "id":"123",
        "datalist":[
            {
                "category":"a",
                "dateTime":"2018-01-01 11:11:11"
            }
            ,
            {
                "category":"bbb",
                "dateTime":"2018-01-01 11:11:11"
            }
        ]
	}`
}

func GzipString(data string) (*bytes.Buffer, error) {
	jsonByte := []byte(data)

	var gzipBuf bytes.Buffer
	gwr := gzip.NewWriter(&gzipBuf)
	defer gwr.Close()

	if _, err := gwr.Write(jsonByte); err != nil {
		log.Print(err)
		return &gzipBuf, err
	}

	return &gzipBuf, nil
}

func sendRequest(url string, postdata string) {
	gzipBuf, err := GzipString(postdata)
	req, err := http.NewRequest("POST", url, gzipBuf)

	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	log.Print(resp.StatusCode)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		resText := string(respBody)
		log.Printf(resText)
	}

}

func waitForPort(waitFunc func()) {
	for !serverAvailable() {
		time.Sleep(time.Second)
		waitFunc()
	}

}

func serverAvailable() bool {
	// GET 호출
	resp, err := http.Get("http://localhost:8080")
	if err != nil {
		log.Print(err)
		return false
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		// 결과 출력
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n", string(data))
	}
	return true
}

func Test_JsonValidate(t *testing.T) {
	t.Log("Test")
}
func Test_WaitServer(t *testing.T) {
	t.Log("start waiting server...")
	waitForPort(func() {
		t.Log("wait a second...")
	})
	t.Log("ok")
}

func Test_Server(t *testing.T) {
	wg.Add(1)
	go func() {

		wg.Done()
	}()

	wg.Add(1)
	go func() {

		wg.Done()
	}()

	wg.Wait()
}
