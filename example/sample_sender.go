package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"bytes"
	"compress/gzip"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
)

var (
	url = flag.String("url", "", "TCP address to listen to")
	cnt = flag.Int("cnt", 1000000, "max retry ")
)

func SendJSONSample() {
	str := `{
        "appId":"aaa",
        "vid":"123",
        "logBody":[
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
	jsonByte := []byte(str)

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

	req, err := http.NewRequest("POST", *url, &gzipBuf)

	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	if err != nil {
		panic(err)
	}

	for i := 0; i < *cnt; i++ {
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
	}

}

func main() {
	SendJSONSample()
}
