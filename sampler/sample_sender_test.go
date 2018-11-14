package main

import "testing"

func Benchmark_GoServer(b *testing.B) {
	c := SamplingConfig{url: "http://localhost/recv", total: b.N, concurrent: 8, datafile: "sample.json"}
	SendJSONSample(&c)
}
