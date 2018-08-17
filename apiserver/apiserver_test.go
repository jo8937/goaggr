package main_test

// https://github.com/valyala/fasthttp/blob/master/examples/helloworldserver/helloworldserver.go

import (
	"testing"
)

func TestSendData(t *testing.T) {
	t.Log("aa")
	for i := 0; i < 100; i++ {
		t.Logf("test %d", i)
	}
}
