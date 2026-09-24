package main

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

// TestBucketsMatchAnton checks the boundaries against Anton Putra's open-source
// Go client when the tutorials repo is available locally.
func TestBucketsMatchAnton(t *testing.T) {
	src, err := os.ReadFile("/home/user/tutorials/lessons/258/client/metrics.go")
	if err != nil {
		t.Skip("tutorials repo not available")
	}
	var want []float64
	for _, s := range regexp.MustCompile(`\d+\.\d+`).FindAllString(string(regexp.MustCompile(`(?s)var buckets.*`).Find(src)), -1) {
		v, _ := strconv.ParseFloat(s, 64)
		want = append(want, v)
	}
	got := buckets()
	if len(got) != len(want) {
		t.Fatalf("got %d buckets, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("bucket %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
