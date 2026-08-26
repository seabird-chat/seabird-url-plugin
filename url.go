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

// extractURLs collects link targets from LinkBlock and regexes the rest out of
// TextBlock. Reading the tree instead of the flattened text avoids matching a
// bare URL twice, since a linkifier turns it into a link whose display text is
// the URL and that flattens to "url (url)". Code blocks are skipped because a
// URL in `code` is being talked about, not linked.
func extractURLs(block *pb.Block) []string {
	return appendURLs(nil, block, false)
}

// appendURLs walks one block. inLink skips the text scan because the link's URL
// already covers its display text.
func appendURLs(urls []string, block *pb.Block, inLink bool) []string {
	if block == nil {
		return urls
	}

	switch inner := block.Inner.(type) {
	case *pb.Block_Text:
		if !inLink {
			urls = append(urls, urlRegex.FindAllString(inner.Text.GetText(), -1)...)
		}

	case *pb.Block_Link:
		if url := inner.Link.GetUrl(); url != "" {
			urls = append(urls, url)
		}

		urls = appendURLs(urls, inner.Link.GetInner(), true)

	case *pb.Block_Italics:
		urls = appendURLs(urls, inner.Italics.GetInner(), inLink)
	case *pb.Block_Bold:
		urls = appendURLs(urls, inner.Bold.GetInner(), inLink)
	case *pb.Block_Underline:
		urls = appendURLs(urls, inner.Underline.GetInner(), inLink)
	case *pb.Block_Strikethrough:
		urls = appendURLs(urls, inner.Strikethrough.GetInner(), inLink)
	case *pb.Block_Spoiler:
		urls = appendURLs(urls, inner.Spoiler.GetInner(), inLink)
	case *pb.Block_Blockquote:
		urls = appendURLs(urls, inner.Blockquote.GetInner(), inLink)
	case *pb.Block_Heading:
		urls = appendURLs(urls, inner.Heading.GetInner(), inLink)

	case *pb.Block_Container:
		for _, child := range inner.Container.GetInner() {
			urls = appendURLs(urls, child, inLink)
		}
	case *pb.Block_List:
		for _, child := range inner.List.GetInner() {
			urls = appendURLs(urls, child, inLink)
		}

	case *pb.Block_InlineCode, *pb.Block_FencedCode, *pb.Block_Timestamp:
		// Nothing to collect.
	}

	return urls
}

// trimURL drops punctuation the regex took from the surrounding sentence. A
// closing paren stays if the URL has an unmatched opening one, for links like
// Wikipedia's /wiki/Go_(game).
func trimURL(raw string) string {
	for raw != "" {
		switch raw[len(raw)-1] {
		case '.', ',', '!', '?', ';', ':', '\'', '"', '<', '>':
		case ')':
			if strings.Count(raw, "(") >= strings.Count(raw, ")") {
				return raw
			}
		default:
			return raw
		}

		raw = raw[:len(raw)-1]
	}

	return raw
}

// cleanURLs trims each URL and drops repeats, keeping the order they appeared
// in, so a message mentioning the same link twice gets one reply.
func cleanURLs(rawurls []string) []string {
	seen := make(map[string]bool, len(rawurls))
	cleaned := make([]string, 0, len(rawurls))

	for _, rawurl := range rawurls {
		rawurl = trimURL(rawurl)
		if rawurl == "" || seen[rawurl] {
			continue
		}

		seen[rawurl] = true
		cleaned = append(cleaned, rawurl)
	}

	return cleaned
}

func (c *Client) messageCallback(source *pb.ChannelSource, text string, rootBlock *pb.Block) {
	// Run all the message matchers in a goroutine to avoid blocking the main
	// URL matching. Note that it may be better to call this serially and let
	// each callback spin up goroutines as needed.
	go func() {
		for _, cb := range c.messageCallbacks {
			cb(c, source, text)
		}
	}()

	// Core synthesizes a text block when the sender used the text-only API, so
	// the walk covers backends without links too. The regex is here for cores
	// old enough to send no blocks at all.
	var rawurls []string
	if rootBlock != nil {
		rawurls = extractURLs(rootBlock)
	} else {
		rawurls = urlRegex.FindAllString(text, -1)
	}

	for _, rawurl := range cleanURLs(rawurls) {
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
					if ok := provider(c, source, u); ok {
						return
					}
				}
			}

			// If we ran through all the providers and didn't reply, try with
			// the default link provider.
			defaultLinkProvider(c, source, raw)
		}(rawurl)
	}
}

// NOTE: This nasty work is done so we ignore invalid ssl certs. We know what
// we're doing, I promise. Famous last words.
//
//nolint:gosec
var client = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	Timeout: 5 * time.Second,
}

func defaultLinkProvider(c *Client, source *pb.ChannelSource, url string) bool {
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
		c.Replyf(source, "Title: %s", title)
		return true
	}

	// URL not handled
	return false
}
