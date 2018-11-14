package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/google/logger"
	"github.com/json-iterator/go"

	"github.com/valyala/fasthttp"
)

var (
	addr             = flag.String("addr", ":8080", "TCP address to listen to")
	fluentNetwork    = flag.String("socket", "unix", "unix or tcp")
	debugMode        = flag.Bool("debug", false, "print request body")
	asyncMode        = flag.Bool("async", true, "send fluentd async mode")
	secondaryLogPath = flag.String("secondary", "/var/log/apiserver.log", "write file when fluentd send fail")
	receiveUri       = flag.String("uri", "/recv", "log receive uri")
	//wg               sync.WaitGroup
	client           *fluent.Fluent
	secondaryLogFile *os.File
	secondaryLogger  *log.Logger
)

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// FILE LOGGER : for Global variable "secondaryLogFile" / "secondaryLogger"
///////////////////////////////////////////////////////////////////////////

func initFileLogger() {
	secondaryLogFile, secondaryLogger = createFileLogger(*secondaryLogPath)
}

func closeFileLogger() {
	secondaryLogFile.Close()
}

func createFileLogger(logPath string) (*os.File, *log.Logger) {
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		logger.Fatalf("Failed to open log file: %v", err)
	}
	lg := log.New(lf, "", 0)

	return lf, lg
	// logger.Info("I'm about to do something!")
	// if err := doSomething(); err != nil {
	// logger.Errorf("Error running doSomething: %v", err)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// FLUENTD :  for Global variable "client" Flientd
///////////////////////////////////////////////////////////////////////////

func initFluentdClient() {
	client = createFluentdClient()
}

func createFluentdClient() *fluent.Fluent {
	log.Printf("Initalize Fluentd... ")
	// https://github.com/fluent/fluent-logger-golang/blob/master/fluent/fluent.go

	var conf fluent.Config

	if *fluentNetwork == "tcp" {
		conf = fluent.Config{FluentPort: 24224, FluentHost: "localhost", Async: *asyncMode, BufferLimit: 1024 * 10} // , WriteTimeout: 3
		log.Print("Use tcp://localhost:24224")
	} else {
		conf = fluent.Config{FluentNetwork: "unix", FluentSocketPath: "/dev/shm/fluentd.sock", Async: *asyncMode, BufferLimit: 1024 * 10} // cover 100kb log. max 102GB Memory. if buffer full, log to secondary file
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

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// HTTP SERVER
///////////////////////////////////////////////////////////////////////////

/**
http 서버를 기동하고 블라킹. 서버 시작 전 fluentd 접속.
*/
func startHTTPServer() {
	//runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()

	initFileLogger()
	defer closeFileLogger()

	initFluentdClient()
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

	currentUri := string(ctx.RequestURI())
	// 지정된 uri 이외에는 그냥 ok 만 보냄
	if currentUri == "/check.php" {
		fmt.Fprintf(ctx, "OK")
		return
	}

	if currentUri != *receiveUri {
		fmt.Fprintf(ctx, "{\"success\":false,\"message\":\"uri not supported\"}")
		return
	}

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

	// TODO : validate JSON  datalist / dateTime / category  must
	// TODO : set client ip
	validationError := validateJSON(dataMap)

	if validationError != nil {
		responseBody := map[string]interface{}{
			"success": true,
			"message": validationError.Error(),
		}
		if jsonstring, ok := json.Marshal(responseBody); ok != nil {
			fmt.Fprintf(ctx, string(jsonstring))
		} else {
			fmt.Fprintf(ctx, "{\"success\":false}")
		}
		return
	}

	if *debugMode {
		log.Printf("requset data : %s", dataMap)
	}

	sendFluentd("apiserver.go", dataMap, ctx.RemoteAddr().String())

	fmt.Fprintf(ctx, "{\"success\":true}")
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Parse Json And Send Fluentd
///////////////////////////////////////////////////////////////////////////

func validateJSON(data map[string]interface{}) error {
	// datalist 는 필수인데 없으면 에러
	datalistInterface, ok := data["datalist"]
	if !ok {
		_, hasCategory := data["category"]
		_, hasDatetime := data["dateTime"]
		_, hasGUID := data["guid"]

		// datalist 가 없어도 1depth 에 category, dateTime, guid 같은 필수컬럼들이 존재한다면
		if hasCategory && hasDatetime && hasGUID {
			return nil
		} else {
			return errors.New("datalist not found. required field also not found")
		}
	}

	// 로그 바디가 있으면 각 하위 항목들도 봐야함..
	datalistTypeVal := reflect.ValueOf(datalistInterface)
	// ) && (kind != reflect.Array)
	if datalistTypeVal.Kind() != reflect.Slice {
		return fmt.Errorf("datalist is not array. it is %s", datalistTypeVal.Elem())
	}

	// map[appId:aaa vid:123 datalist:[map[category:a dateTime:2018-01-01 11:11:11] map[category:bbb dateTime:2018-01-01 11:11:11]]]
	for i := 0; i < datalistTypeVal.Len(); i++ {
		row := datalistTypeVal.Index(i).Interface()

		rowVal, ok := row.(map[string]interface{})
		// ) && (kind != reflect.Array)

		if !ok {
			return fmt.Errorf("datalist [%d] row is not map. %s", i, rowVal)
		}

		if _, ok := rowVal["category"]; !ok {
			return fmt.Errorf("datalist [%d] category not found ", i)
		}
		if _, ok := rowVal["dateTime"]; !ok {
			return fmt.Errorf("datalist [%d] datetime not found ", i)
		}
	}

	// datalist 엘리먼트들을 순회. 근데 category 나 dateTime 이 없다면 에러...

	//err = fmt.Errorf("Invalid parameter [%s]", param)
	return nil
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
실제 로그를 fluentd 로 전달. 비동기로 동작. async await 걸라고 했는데, fluentd 내부에서 이미 async 비동기 처리를 하므로 내쪽에서 할수 있는게 없음.
*/
func sendFluentd(tag string, data map[string]interface{}, clientIp string) {

	data["regdate"] = time.Now().UTC().Format("2006-01-02 15:04:05")
	data["ip"] = clientIp

	//wg.Add(1)
	//go func(data map[string]interface{}) {
	//	defer wg.Done()
	flientdErr := client.Post(tag, data)
	// error := logger.PostWithTime(tag, time.Now(), data)
	// fluentd 에 밀어넣는데 뭔가 에러가 났으면....
	if flientdErr != nil {
		// stdout 에 에러로그 찍음
		log.Print("fail to send fluentd")
		log.Print(flientdErr)

		// 원본로그를 남김.
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		// json 만드는거가 에러났다면 이건 어쩔수가 없다 그냥 std 에 에러찍고 넘어감
		if JSONString, err := json.Marshal(data); err != nil {
			log.Print(err)
			log.Printf("%s", data)
		} else {
			// JSON 을 파일에 기록
			secondaryLogger.Println(string(JSONString))
		}
	}
	//time.Sleep(time.Second * 3)
	//}(data)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// main
func main() {
	startHTTPServer()
}
