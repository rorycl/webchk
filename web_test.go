package main

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"

	"github.com/google/go-cmp/cmp"
)

func TestGetMatches(t *testing.T) {
	tests := []struct {
		body        []byte
		searchTerms []string
		hits        int
		strResults  string // SearchMatch.String()
	}{
		{
			body:        []byte(""),
			searchTerms: []string{},
			hits:        0,
		},
		{
			body:        []byte("hi there"),
			searchTerms: []string{},
			hits:        0,
		},
		{
			body:        []byte("hi there"),
			searchTerms: []string{"hi"},
			hits:        1,
		},
		{
			body:        []byte("hi there old man"),
			searchTerms: []string{"hi", "old"},
			hits:        2,
		},
		{
			// note that only one hit is reported a line
			body:        []byte("there there old man"),
			searchTerms: []string{"there"},
			hits:        1,
		},
		{
			body:        []byte("there\nthere old man"),
			searchTerms: []string{"there"},
			hits:        2,
		},
		{
			body:        []byte("there\nthere old man\nmatch string\n---"),
			searchTerms: []string{"match string"},
			hits:        1,
			strResults:  "line:   3 match: match string", // line 3, search term
		},
		{
			body:        []byte("there there old man"),
			searchTerms: []string{"zero"},
			hits:        0,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			matches := getMatches(tt.body, tt.searchTerms)
			if got, want := len(matches), tt.hits; got != want {
				t.Errorf("got %d != want %d", got, want)
				t.Logf("%#v", tt)
			}
			if tt.strResults != "" && len(matches) > 0 {
				if got, want := fmt.Sprint(matches[0]), tt.strResults; got != want {
					t.Errorf("string match got %s != want %s", got, want)

				}
			}
		})
	}
}

// could use github.com/google/go-cmp/cmp/cmpopts
func TestGetLinks(t *testing.T) {
	tests := []struct {
		body  []byte
		url   string
		links []string
		isErr bool
	}{
		{
			body:  []byte(""),
			url:   "https://example.com/query",
			links: []string{},
			isErr: false,
		},
		{
			body:  []byte(`<html><body><a href="/one">one</a></html>`),
			url:   "https://e.com/q",
			links: []string{"https://e.com/one"},
			isErr: false,
		},
		{
			body:  []byte(`<html><body><a href="../one?testme">one</a></html>`),
			url:   "https://e.com/q",
			links: []string{"https://e.com/one"},
			isErr: false,
		},
		{
			body:  []byte(`<html><body><a mailto="x@y.com">mail</a></html>`),
			url:   "https://e.com/q",
			links: []string{}, // skipped
			isErr: false,
		},
		{
			body:  []byte(`<html><body><a href="one">one</a><p>ok</p><a href="two">two</a></html>`),
			url:   "https://e.com/q",
			links: []string{"https://e.com/one", "https://e.com/two"},
			isErr: false,
		},
		{
			body:  []byte(`<html><body><a href="two">two</a><p>ok</p><a href="one">one</a></html>`),
			url:   "https://e.com/q",
			links: []string{"https://e.com/one", "https://e.com/two"}, // sorted
			isErr: false,
		},
		{
			body:  []byte(`<html><body><a href="two">two</a><p>ok</p><a href="two">two</a></html>`),
			url:   "https://e.com/q",
			links: []string{"https://e.com/two"}, // compacted
			isErr: false,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			url, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("could not parse url %v", err)
			}
			links, err := getLinks(tt.body, url)
			if err != nil {
				if !tt.isErr {
					t.Fatalf("unexpected err %v", err)
				} else {
					return
				}
			}
			if !cmp.Equal(links, tt.links) {
				t.Errorf("got %v != want %v", links, tt.links)
				t.Logf("%s : %s", string(tt.body), tt.links)
			}
		})
	}
}

func TestGetMakeClient(t *testing.T) {

	tp := func(td string) time.Duration {
		d, err := time.ParseDuration(td)
		if err != nil {
			t.Fatalf("time parsing error: %v", err)
		}
		return d
	}

	tests := []struct {
		name        string
		httpWorkers int
		httpTimeout time.Duration
		wantWorkers int
		wantTimeout time.Duration
	}{
		{"defaults", 0, time.Duration(0), HTTPWORKERS, HTTPTIMEOUT}, // use defaults
		{"custom_1", 1, tp("1m20s"), 1, tp("1m20s")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewGetClient(tt.httpWorkers, tt.httpTimeout)
			thisTransport := d.client.Transport.(*http.Transport)
			if got, want := thisTransport.MaxConnsPerHost, tt.wantWorkers; got != want {
				t.Errorf("httpworkers got %v != want %v", got, want)
			}
			if got, want := d.client.Timeout, tt.wantTimeout; got != want {
				t.Errorf("timeout got %v != want %v", got, want)
			}
		})
	}
}

func TestGetURL(t *testing.T) {

	serverResponseText := "hello world"
	serverHeader := "text/html; charset=utf-8"
	serverOK := true
	httpHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if !serverOK {
				w.WriteHeader(http.StatusForbidden)
			}
			w.Header().Set("Content-Type", serverHeader)
			fmt.Fprintln(w, serverResponseText)
		},
	)
	server := httptest.NewTLSServer(httpHandler)
	defer server.Close()
	server.Config.ReadTimeout = 200 * time.Millisecond

	// indirect getLinks and getMatch
	var linkError error = nil
	var aLinkError = errors.New("link error")
	getLinker := func(body []byte, url *url.URL) ([]string, error) {
		return []string{}, linkError
	}
	getMatcher := func(body []byte, searchTerms []string) []SearchMatch {
		return []SearchMatch{}
	}

	// make new get client
	g := getClient{}
	g.client = server.Client()
	g.client.Timeout = 300 * time.Millisecond
	g.getLinks = getLinker
	g.getMatches = getMatcher

	tests := []struct {
		// server
		serverOK     bool
		serverText   string
		serverHeader string
		// fn
		linkError error
		// input
		url         string
		searchTerms []string
		// results
		result Result
	}{
		{
			serverOK:     true,
			serverText:   "no links",
			serverHeader: "text/html; charset=utf-8",
			url:          server.URL,
			searchTerms:  []string{}, // no searches
			result: Result{
				url:    server.URL,
				status: 200,
				err:    nil,
			},
		},
		{
			serverOK:     true,
			serverText:   "no links; wrong server Header",
			serverHeader: "application/json",
			url:          server.URL,
			searchTerms:  []string{}, // no searches
			result: Result{
				url:    server.URL,
				status: 200,
				err:    NonHTMLPageType,
			},
		},
		{
			serverOK:     false,
			serverText:   "should get 403 error",
			serverHeader: "text/html; charset=utf-8",
			url:          server.URL,
			searchTerms:  []string{}, // no searches
			result: Result{
				url:    server.URL,
				status: 403,
				err:    StatusNotOk,
			},
		},
		{
			serverOK:     true,
			serverText:   "", // empty
			serverHeader: "text/html; charset=utf-8",
			url:          server.URL,
			searchTerms:  []string{"hi", "there"},
			result: Result{
				url:    server.URL,
				status: 200,
				err:    nil,
			},
		},
		{
			serverOK:     true,
			serverText:   "link error",
			serverHeader: "text/html; charset=utf-8",
			linkError:    aLinkError,
			url:          server.URL,
			searchTerms:  []string{"hi", "there"},
			result: Result{
				url:    server.URL,
				status: 200,
				err:    aLinkError,
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			// setup server
			serverOK = tt.serverOK
			serverHeader = tt.serverHeader
			serverResponseText = tt.serverText
			if tt.linkError != nil { // fake error from getLinker function
				linkError = tt.linkError
			}

			result, _ := g.get(tt.url, "/referrer", tt.searchTerms)

			if result.err != tt.result.err {
				if !errors.Is(result.err, tt.result.err) {
					t.Errorf("error mismatch want %v got %v", tt.result.err, result.err)
				}
			}
			// result is unexported, so check fields
			if diff := cmp.Diff(tt.result.status, result.status); diff != "" {
				t.Errorf("result status mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.result.url, result.url); diff != "" {
				t.Errorf("result url mismatch (-want +got):\n%s", diff)
			}

		})
	}
}
