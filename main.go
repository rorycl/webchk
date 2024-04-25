// webchk is a command for searching all the pages on a website for one
// or more search phrases. The search phrase matches are made in
// lowercase and a naive search is made including all of the markup
// (instead of using, for example, goquery).
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// linkError is a type for sentinel errors
type linkError string

// Error meets the error interface requirement
func (err linkError) Error() string {
	return string(err)
}

const (
	// GOWORKERS is the number of worker goroutines to start
	GOWORKERS = 8
	// HTTPWORKERS is the number of concurrent web queries to run
	HTTPWORKERS = 16
	// HTTPTIMEOUT is the longest a web connection will stay open
	HTTPTIMEOUT time.Duration = 3 * time.Second
	// Sentinel error for non html pages
	NonHTMLPageType = linkError("NonHTMLPageType")
	StatusNotOk     = linkError("StatusNotOk")
)

var (
	// Client is a shared http.Client with a specific configuration
	// including Transport config
	Client *http.Client
	// url suffixes to skip
	URLSuffixesToSkip = []string{".png", ".jpg", ".jpeg", ".heic", ".svg"}
)

// Result is provided by a worker
type Result struct {
	url     string        // full url
	status  int           // http statuscode if not 200
	matches []SearchMatch // matches from this URL
	err     error
}

// SearchMatch is a record of a search match in an html file
type SearchMatch struct {
	line  int    // line number
	match string // the match
}

// String prints a SearchMatch
func (s SearchMatch) String() string {
	return fmt.Sprintf("%3d : %s", s.line, s.match)
}

// Links is a slice of urls protected by a sync.Mutex
type Links struct {
	links []string
	sync.Mutex
}

// Add adds an item to a links slice protected by a Mutex
func (l *Links) Add(u string) {
	l.Lock()
	defer l.Unlock()
	l.links = append(l.links, u)
}

// Pop removes the first item from a links slice, protected by a Mutex
func (l *Links) Pop() string {
	l.Lock()
	defer l.Unlock()
	if len(l.links) == 0 {
		return ""
	}
	u := l.links[0]
	l.links = l.links[1:]
	return u
}

// getter is a function for launching worker goroutines to process
// getURL functions to produce results
func getter(baseURL string, searchTerms []string) <-chan Result {

	results := make(chan Result)
	follow := followURLs(baseURL)

	// links is a buffer for processing links
	links := Links{}
	links.Add(baseURL)

	var wg sync.WaitGroup
	for i := 0; i < GOWORKERS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initialised := false
			for {
				url := links.Pop()
				switch {
				case initialised && url == "": // assume there are no more links to process
					return

				case url == "":
					time.Sleep(500 * time.Millisecond)
					continue
				}
				initialised = true
				thisResult, theseLinks := getURL(url, searchTerms)
				results <- thisResult
				for _, l := range theseLinks {
					if follow(l) {
						links.Add(l)
					}
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}

// getURL gets a URL, reporting a status if not 200, extracts the links
// from the page and reports if there are any matches to the
// searchTerms.
func getURL(url string, searchTerms []string) (Result, []string) {
	r := Result{url: url}
	links := []string{}

	resp, err := Client.Get(url)
	if err != nil {
		r.err = err
		return r, links
	}
	if resp.StatusCode != http.StatusOK {
		r.status = resp.StatusCode
		r.err = StatusNotOk
		return r, links
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		r.err = NonHTMLPageType
		return r, links
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body) // read into body in order to parse it twice
	if err != nil {
		r.err = fmt.Errorf("file reading error: %w", err)
		return r, links
	}

	// Parse the body into an x/html tree.
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		r.err = fmt.Errorf("could not parse file: %w", err)
		return r, links
	}

	// Find any links in the html body.
	var visit func(n *html.Node) []string // declare as recursive
	visit = func(n *html.Node) []string {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					linkURL, err := resp.Request.URL.Parse(a.Val)
					if err != nil {
						continue // ignore bad urls
					}
					linkURL.RawQuery, linkURL.Fragment = "", "" // remove items after path
					link := linkURL.String()
					links = append(links, link)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			links = visit(c)
		}
		slices.Sort(links)
		links = slices.Compact(links)
		return links
	}
	links = visit(doc)

	// Find if any of the search terms match text in the body.
	// Note that this match is case insensitive.
	if len(searchTerms) == 0 {
		return r, links
	}
	r.matches = []SearchMatch{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		for _, st := range searchTerms {
			if strings.Contains(strings.ToLower(scanner.Text()), strings.ToLower(st)) {
				r.matches = append(r.matches, SearchMatch{lineNo, st})
			}
		}
	}

	return r, links
}

// followURLs is a closure which returns true if a url has not been seen
// before and the provided url matches the baseURL and does not match
// one of the provided URLSuffixes
func followURLs(baseURL string) func(u string) bool {
	uniqueURLs := map[string]bool{baseURL: true}
	return func(u string) bool {
		u = strings.TrimSuffix(u, "/")
		if !strings.Contains(u, baseURL) {
			return false
		}
		if uniqueURLs[u] {
			return false
		}
		for _, skip := range URLSuffixesToSkip {
			if strings.HasSuffix(u, skip) {
				return false
			}
		}
		uniqueURLs[u] = true
		return true
	}
}

func main() {

	baseURL := "https://rotamap.net"

	Client = &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: HTTPWORKERS,
		},
		Timeout: HTTPTIMEOUT,
	}

	queries := []string{"campbell-lange"}
	results := getter(baseURL, queries)

	pages := 0
	for r := range results {
		pages++
		switch r.err {
		case NonHTMLPageType:
			continue
		case StatusNotOk:
			fmt.Printf("%s : status %d\n", r.url, r.status)
			continue
		default:
			if r.err != nil {
				fmt.Printf("%s : error %v\n", r.url, r.err)
				continue
			}
		}

		// fmt.Printf("%s\n", r.url)

		// matches
		if len(r.matches) > 0 {
			fmt.Printf("%s\n", r.url)
			for _, m := range r.matches {
				fmt.Printf("> match %s\n", m)
			}
		}
	}
	fmt.Println("processed", pages, "pages")
}
