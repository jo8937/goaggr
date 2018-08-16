package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/json-iterator/go"

	//"compress/gzip"
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

func requestHandler(ctx *fasthttp.RequestCtx) {
	log.Printf("Raw request is:\n---CUT---\n%s\n---CUT---", &ctx.Request)

	data := parseJSONLog(ctx.PostBody())

	go sendFluentd(data)

	ctx.SetContentType("application/json; charset=utf8")
	fmt.Fprintf(ctx, "{}")

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

func sendFluentd(data map[string]interface{}) {

	tag := "debug.golang"
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
	data := parseJSONLog([]byte(str))
	log.Printf("%s", data["logBody"])

	//client = createFluentdClient()
	//defer closeFluentd()
	// TOSO send http

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
