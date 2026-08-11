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

// extractURLsFromBlocks recursively walks a block tree and extracts all URLs
// from LinkBlock nodes.
func extractURLsFromBlocks(block *pb.Block) []string {
	if block == nil {
		return nil
	}

	var urls []string

	switch inner := block.Inner.(type) {
	case *pb.Block_Link:
		// Found a link block - extract the URL
		if inner.Link != nil && inner.Link.Url != "" {
			urls = append(urls, inner.Link.Url)
		}
		// Also check if the inner block contains more links
		if inner.Link != nil && inner.Link.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Link.Inner)...)
		}

	case *pb.Block_Italics:
		if inner.Italics != nil && inner.Italics.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Italics.Inner)...)
		}

	case *pb.Block_Bold:
		if inner.Bold != nil && inner.Bold.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Bold.Inner)...)
		}

	case *pb.Block_Underline:
		if inner.Underline != nil && inner.Underline.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Underline.Inner)...)
		}

	case *pb.Block_Strikethrough:
		if inner.Strikethrough != nil && inner.Strikethrough.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Strikethrough.Inner)...)
		}

	case *pb.Block_Spoiler:
		if inner.Spoiler != nil && inner.Spoiler.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Spoiler.Inner)...)
		}

	case *pb.Block_Blockquote:
		if inner.Blockquote != nil && inner.Blockquote.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Blockquote.Inner)...)
		}

	case *pb.Block_Heading:
		if inner.Heading != nil && inner.Heading.Inner != nil {
			urls = append(urls, extractURLsFromBlocks(inner.Heading.Inner)...)
		}

	case *pb.Block_Container:
		if inner.Container != nil {
			for _, childBlock := range inner.Container.Inner {
				urls = append(urls, extractURLsFromBlocks(childBlock)...)
			}
		}

	case *pb.Block_List:
		if inner.List != nil {
			for _, childBlock := range inner.List.Inner {
				urls = append(urls, extractURLsFromBlocks(childBlock)...)
			}
		}

	// Text blocks don't contain URLs in the block structure
	case *pb.Block_Text, *pb.Block_InlineCode, *pb.Block_FencedCode, *pb.Block_Timestamp:
		// No nested blocks to process
	}

	return urls
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

	// Use block-based URL extraction if blocks are available, otherwise fall back to regex
	var rawurls []string
	if rootBlock != nil {
		rawurls = extractURLsFromBlocks(rootBlock)
	} else {
		rawurls = urlRegex.FindAllString(text, -1)
	}

	for _, rawurl := range rawurls {
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
