package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/json-iterator/go"

	"sync"

	"github.com/valyala/fasthttp"
)

var (
	addr     = flag.String("addr", ":8080", "TCP address to listen to")
	compress = flag.Bool("compress", false, "Whether to enable transparent response compression")
	client   *fluent.Fluent
	wg       sync.WaitGroup
)

func createFluentdClient() *fluent.Fluent {
	log.Printf("Connect Fluentd... ")
	c, fluentderr := fluent.New(fluent.Config{FluentPort: 24224, FluentHost: "localhost"})
	if fluentderr != nil {
		panic(fluentderr)
	}
	return c
}
func closeFluentd() {
	log.Print("close fluentd")
	client.Close()
}

/**
http 서버를 기동하고 블라킹. 서버 시작 전 fluentd 접속.
*/
func startHTTPServer() {
	client = createFluentdClient()
	defer closeFluentd()
	flag.Parse()

	handler := requestHandler
	if *compress {
		handler = fasthttp.CompressHandler(handler)
	}

	log.Printf("Start Server... %s", *addr)

	if err := fasthttp.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

/*
	실제 리퀘스츠 처리부
*/
func requestHandler(ctx *fasthttp.RequestCtx) {
	//log.Printf("Raw request is:\n---CUT---\n%s\n---CUT---", &ctx.Request)
	//"Content-Encoding"
	//	if ctx.

	reqContentEncoding := ctx.Request.Header.Peek("Content-Encoding")
	var dataMap map[string]interface{}

	if reqContentEncoding != nil && string(reqContentEncoding) == "gzip" {
		gzipData := ctx.PostBody()

		var unzipDataBuf bytes.Buffer

		fasthttp.WriteGunzip(&unzipDataBuf, gzipData)

		dataMap = parseJSONLog(unzipDataBuf.Bytes())
	} else {
		dataMap = parseJSONLog(ctx.PostBody())
	}

	log.Printf("requset data : %s", dataMap)

	go sendFluentd("debug.aaa", dataMap)

	ctx.SetContentType("application/json; charset=utf8")
	fmt.Fprintf(ctx, "{\"success\":true}")

}

func parseJSONLog(bytes []byte) map[string]interface{} {
	var datamap map[string]interface{}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(bytes, &datamap); err != nil {
		log.Printf("JSON Parse ERROR : %s", bytes)
		log.Fatal(err)
	}
	//log.Printf("%+v", datamap)
	//log.Printf("%s", bytes)

	return datamap
}

func sendFluentd(tag string, data map[string]interface{}) {

	wg.Add(1)
	go func(data map[string]interface{}) {
		defer wg.Done()
		error := client.Post(tag, data)
		// error := logger.PostWithTime(tag, time.Now(), data)
		if error != nil {
			//panic(error)
			// TODO : file write
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			if JSONString, err := json.Marshal(data); err != nil {
				log.Fatal(JSONString)
			} else {
				log.Fatal("fail to send fluentd")
			}
		}
		//time.Sleep(time.Second * 3)
	}(data)
}

func Example_JSONSend() {
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
	data := parseJSONLog(jsonByte)
	log.Printf("%s", data["logBody"])

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

	req, err := http.NewRequest("POST", "http://localhost:8080/v2/recv", &gzipBuf)

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
}

func Example_StartServerAndSendData() {

	wg.Add(1)
	go func() {
		startHTTPServer()
		wg.Done()
	}()

	time.Sleep(time.Second)
	wg.Add(1)

	go Example_JSONSend()
	wg.Wait()
}

func main() {

	Example_StartServerAndSendData()
}
