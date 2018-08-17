package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"bytes"
	"flag"
	"fmt"
	"log"

	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/json-iterator/go"

	"sync"

	"github.com/valyala/fasthttp"
)

var (
	addr = flag.String("addr", ":8080", "TCP address to listen to")
	//compress = flag.Bool("compress", false, "Whether to enable transparent response compression")
	//compress = flag.Bool("compress", false, "Whether to enable transparent response compression")
	fluentNetwork = flag.String("socket", "unix", "unix or tcp")
	DEBUG_MODE    = flag.Bool("debug", false, "print request body")
	client        *fluent.Fluent
	wg            sync.WaitGroup
)

func createFluentdClient() *fluent.Fluent {
	log.Printf("Initalize Fluentd... ")
	// https://github.com/fluent/fluent-logger-golang/blob/master/fluent/fluent.go

	var conf fluent.Config

	if *fluentNetwork == "tcp" {
		conf = fluent.Config{FluentPort: 24224, FluentHost: "localhost", Async: true}
		log.Print("Use tcp://localhost:24224")
	} else {
		conf = fluent.Config{FluentNetwork: "unix", FluentSocketPath: "/dev/shm/fluentd.sock", Async: true}
		log.Print("Use unix:///dev/shm/fluentd.sock")
	}

	c, fluentderr := fluent.New(conf)

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
	flag.Parse()

	client = createFluentdClient()
	defer closeFluentd()

	handler := requestHandler

	log.Printf("Start fasthttp Web Server... %s", *addr)

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
	ctx.SetContentType("application/json; charset=utf8")

	reqContentEncoding := ctx.Request.Header.Peek("Content-Encoding")
	var dataMap map[string]interface{}

	postbody := ctx.PostBody()

	if postbody == nil {
		fmt.Fprintf(ctx, "{\"success\":false,\"message\":\"post body not found\"}")
		return
	}

	if reqContentEncoding != nil && string(reqContentEncoding) == "gzip" {
		var unzipDataBuf bytes.Buffer

		fasthttp.WriteGunzip(&unzipDataBuf, postbody)

		dataMap = parseJSONLog(unzipDataBuf.Bytes())
	} else {
		dataMap = parseJSONLog(postbody)
	}

	if dataMap == nil {
		fmt.Fprintf(ctx, "{\"success\":false,\"message\":\"json parse error\"}")
		return
	}

	// TODO : validate JSON  logBody / dateTime / category  must
	// TODO : set client ip

	if *DEBUG_MODE {
		log.Printf("requset data : %s", dataMap)
	}

	go sendFluentd("debug.aaa", dataMap)

	fmt.Fprintf(ctx, "{\"success\":true}")
}

func parseJSONLog(bytes []byte) map[string]interface{} {
	var datamap map[string]interface{}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(bytes, &datamap); err != nil {
		//log.Printf("JSON Parse ERROR : %s", bytes)
		log.Print(err)
		return nil
	}
	//log.Printf("%+v", datamap)
	//log.Printf("%s", bytes)

	return datamap
}

/**
실제 로그를 fluentd 로 전달. 비동기로 동작
*/
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

func main() {
	startHTTPServer()
}
