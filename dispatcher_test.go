package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestLinkError(t *testing.T) {
	err := linkError("link error")
	if got, want := err.Error(), "link error"; got != want {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestFollowURLs(t *testing.T) {

	tests := []struct {
		url string
		ok  bool
	}{
		// beware order is important
		{"http://x.com", false},  // base url should fail
		{"http://x.com/", false}, // base url should fail with slash
		{"http://x.com/ok/", true},
		{"http://x.com/ok/", false},   // seen before
		{"http://x.com/1.svg", false}, // svg
		{"http://x.com/1.png", false}, // png
		{"http://x.com/uniqe", true},  // unique
	}

	// init
	f := followURLs("http://x.com")

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			if got, want := f(tt.url), tt.ok; got != want {
				t.Errorf("%s got %t want %t", tt.url, got, want)
			}
		})
	}
}

func TestGetter(t *testing.T) {

	// getURLer is an indirector: see dispatcher.go
	links := []string{}
	getURLer = func(url string, searchTerms []string) (Result, []string) {
		return Result{
			url:     "https://example.com",
			status:  200,
			matches: []SearchMatch{},
		}, links
	}

	resultCollector := func() int {
		i := 0
		for _ = range Dispatcher("https://example.com", []string{}) {
			i++
		}
		return i
	}

	// build urls from strings
	prefixer := func(s ...string) []string {
		for i, x := range s {
			s[i] = "https://example.com/" + x
		}
		return s
	}

	tests := []struct {
		workers        int
		linkbuffersize int
		links          []string
		resultNo       int
	}{
		{
			workers:        1,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultNo:       3, // base url + 2 links
		},
		{
			// fails with not enough room in the buffer
			workers:        1,
			linkbuffersize: 1,
			links:          prefixer([]string{"1", "2"}...),
			resultNo:       1, // base url, no links
		},
		{
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultNo:       3,
		},
		{
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1"}...),
			resultNo:       2,
		},
		{
			workers:        1,
			linkbuffersize: 20,
			links:          prefixer(strings.Fields("a b c d e f g h i j k l m n o p")...),
			resultNo:       17, // base + 16
		},
		{
			workers:        50,
			linkbuffersize: 10,
			links:          prefixer(strings.Fields("a b c d e f")...),
			resultNo:       7, // base + 6
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			GOWORKERS = tt.workers
			LINKBUFFERSIZE = tt.linkbuffersize
			links = tt.links
			if got, want := resultCollector(), tt.resultNo; got != want {
				t.Errorf("got %d want %d results", got, want)
			}
		})
	}
}
