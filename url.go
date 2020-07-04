package url

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/seabird-chat/seabird-go/pb"
)

// NOTE: This isn't perfect in any sense of the word, but it's pretty close
// and I don't know if it's worth the time to make it better.
var (
	urlRegex     = regexp.MustCompile(`https?://[^ ]+`)
	newlineRegex = regexp.MustCompile(`\s*\n\s*`)
)

func (c *Client) messageCallback(event *pb.MessageEvent) {
	// Run all the message matchers in a goroutine to avoid blocking the main
	// URL matching. Note that it may be better to call this serially and let
	// each callback spin up goroutines as needed.
	go func() {
		for _, cb := range c.messageCallbacks {
			cb(c, event)
		}
	}()

	for _, rawurl := range urlRegex.FindAllString(event.Text, -1) {
		go func(raw string) {
			u, err := url.ParseRequestURI(raw)
			if err != nil {
				return
			}

			// Strip the last character if it's a slash
			u.Path = strings.TrimRight(u.Path, "/")

			targets := []string{u.Host}

			// If there was a www, we fall back to no www This is not perfect,
			// but it will fix a number of issues Alternatively, we could
			// require the linkifiers to register multiple times
			if strings.HasPrefix(u.Host, "www.") {
				targets = append(targets, strings.TrimPrefix(u.Host, "www."))
			}

			for _, host := range targets {
				for _, provider := range c.callbacks[host] {
					if ok := provider(c, event, u); ok {
						return
					}
				}
			}

			// If we ran through all the providers and didn't reply, try with
			// the default link provider.
			defaultLinkProvider(c, event, raw)
		}(rawurl)
	}
}

// NOTE: This nasty work is done so we ignore invalid ssl certs. We know what
// we're doing.
//nolint:gosec
var client = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	Timeout: 5 * time.Second,
}

func defaultLinkProvider(c *Client, event *pb.MessageEvent, url string) bool {
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	// We search the first 1K and if a title isn't in there, we deal with it
	z, err := html.Parse(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		log.Printf("Failed to grab URL: %s", err)
		return false
	}

	// Scrape the tree for the first title node we find
	n, ok := scrape.Find(z, scrape.ByTag(atom.Title))

	// If we got a result, pull the text from it
	if ok {
		title := newlineRegex.ReplaceAllLiteralString(scrape.Text(n), " ")
		c.Replyf(event.Source, "Title: %s", title)
		return true
	}

	// URL not handled
	return false
}
