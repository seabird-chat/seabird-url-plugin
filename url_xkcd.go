package url

import (
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/seabird-chat/seabird-go/pb"
	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var xkcdRegex = regexp.MustCompile(`^/([^/]+)$`)
var xkcdPrefix = "[XKCD]"

func NewXKCDProvider() *XKCDProvider {
	return &XKCDProvider{}
}

type XKCDProvider struct{}

func (p *XKCDProvider) GetCallbacks() map[string]URLCallback {
	return map[string]URLCallback{
		"xkcd.com": handleXKCD,
	}
}

func (p *XKCDProvider) GetMessageCallback() MessageCallback {
	return nil
}

func handleXKCD(c *Client, event *pb.MessageEvent, u *url.URL) bool {
	if u.Path != "" && !xkcdRegex.MatchString(u.Path) {
		return false
	}

	resp, err := http.Get(u.String())
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
		return false
	}

	// Scrape the tree for the first title node we find
	n, ok := scrape.Find(z, scrape.ById("comic"))
	if !ok {
		return false
	}

	n, ok = scrape.Find(n, scrape.ByTag(atom.Img))
	if !ok {
		return false
	}

	c.Replyf(event.Source, "%s %s: %s", xkcdPrefix, scrape.Attr(n, "alt"), scrape.Attr(n, "title"))

	return true
}
