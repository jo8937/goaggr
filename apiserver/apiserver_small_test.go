package main

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestFileLogger(t *testing.T) {
	t.Log("file logger ")
	initFileLogger()
	secondaryLogger.Println("test")
	closeFileLogger()
}

func TestMakeError(t *testing.T) {
	var ErrInvalidParam = fmt.Errorf("Invalid parameter [%s]", "aaa")
	t.Log(ErrInvalidParam.Error())
}

func TestValidateJSON(t *testing.T) {
	str := GetSampleLogJSON()
	data := parseJSONLog([]byte(str))
	err := validateJSON(data)
	if err != nil {
		t.Error(err)
	}
}

func TestParseJSONLog(t *testing.T) {
	str := GetSampleLogJSON()
	data := parseJSONLog([]byte(str))
	if data == nil {
		t.Error("json parse error")
	}
	t.Log(data)

	data = parseJSONLog([]byte("{aaa}"))
	if data != nil {
		t.Error("invalid json parsed ")
	}

	t.Log("OK")
}

func TestEscape(t *testing.T) {
	resstring, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": "cript",
	})
	t.Log(string(resstring))
	//	t.Logf("%s", strings.Replace("asdfsdf\"asdf\"asdfa\\\"sdf", "\"", "\\\"", -1))
}

func TestTimeFormat(t *testing.T) {
	t.Log(time.Now().Format("2006-01-02 15:04:05"))
	t.Log(time.Now().UTC().Format("2006-01-02 15:04:05"))
}
