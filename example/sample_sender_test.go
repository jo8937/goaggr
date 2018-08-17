package main

import "testing"

func Benchmark_GoServer(b *testing.B) {
	c := SamplingConfig{url: "http://localhost:8080", total: b.N, concurrent: 8, datafile: "sample.json"}
	SendJSONSample(&c)
}
