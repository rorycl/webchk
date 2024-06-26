// web.go gets web pages, extracting links from the page
// and searching the content for searchterms.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// getClient encapsulates an http.Client and the functions used against
// that client, which are parameterised to allow for convenient swapping
// out during testing
type getClient struct {
	client     *http.Client
	getURL     func(url, referrer string, searchTerms []string) (Result, []string)
	getLinks   func(body []byte, url *url.URL) ([]string, error)
	getMatches func(body []byte, searchTerms []string) []SearchMatch
}

// NewGetClient initialises a new getClient.
func NewGetClient(httpWorkers int, httpTimeout time.Duration) *getClient {
	if httpWorkers == 0 {
		httpWorkers = HTTPWORKERS
	}
	if httpTimeout == 0 {
		httpTimeout = HTTPTIMEOUT
	}
	g := getClient{}
	g.client = &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: httpWorkers,
		},
		Timeout: httpTimeout,
	}
	g.getURL = g.get
	g.getLinks = getLinks
	g.getMatches = getMatches
	return &g
}

// Result is url result provided by a call to a web page
type Result struct {
	url, referrer string        // full url and referrer
	status        int           // http statuscode if not 200
	matches       []SearchMatch // search term matches from this URL
	err           error
}

// SearchMatch is a record of a search term match in an html file
type SearchMatch struct {
	line  int    // line number
	match string // the match term
}

// String prints a SearchMatch
func (s SearchMatch) String() string {
	return fmt.Sprintf("line: %3d match: %s", s.line, s.match)
}

// get gets a URL, reporting a status if not 200, extracts the links
// from the page and reports if there are any matches to the
// searchTerms.
func (g *getClient) get(url, referrer string, searchTerms []string) (Result, []string) {
	r := Result{
		url:      url,
		referrer: referrer,
		matches:  []SearchMatch{},
	}
	links := []string{}

	resp, err := g.client.Get(url)
	if err != nil {
		r.err = err
		return r, links
	}
	r.status = resp.StatusCode
	if r.status != http.StatusOK {
		r.err = StatusNotOk
		return r, links
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		r.err = NonHTMLPageType
		return r, links
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body) // read into body for multiple uses
	if err != nil {
		r.err = fmt.Errorf("file reading error: %w", err)
		return r, links
	}

	links, err = g.getLinks(body, resp.Request.URL)
	if err != nil {
		r.err = fmt.Errorf("links error: %w", err)
		return r, links
	}

	r.matches = g.getMatches(body, searchTerms)

	return r, links
}

// getLinks extracts the links from an html page by parsing it in to an
// x/html tree returning a slice of links or error. The tree parser is
// taken from the blue book.
func getLinks(body []byte, url *url.URL) ([]string, error) {
	links := []string{}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		err = fmt.Errorf("could not parse file: %w", err)
		return links, err
	}
	// Find any links
	var visit func(n *html.Node) []string // declare here as recursive
	visit = func(n *html.Node) []string {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					linkURL, err := url.Parse(a.Val)
					if err != nil {
						continue // ignore bad urls
					}
					linkURL.RawQuery, linkURL.Fragment = "", "" // remove items after path
					link := linkURL.String()
					link = strings.TrimSpace(strings.TrimSuffix(link, "/"))
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

	return links, nil
}

// getMatches finds if any of the search terms match text in the
// body. Matching is case insensitive.
func getMatches(body []byte, searchTerms []string) []SearchMatch {
	matches := []SearchMatch{}
	if len(searchTerms) == 0 {
		return matches
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		for _, st := range searchTerms {
			if strings.Contains(strings.ToLower(scanner.Text()), strings.ToLower(st)) {
				matches = append(matches, SearchMatch{lineNo, st})
			}
		}
	}
	return matches
}
