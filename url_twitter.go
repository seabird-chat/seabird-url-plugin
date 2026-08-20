package url

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/seabird-chat/seabird-go/pb"

	"github.com/seabird-chat/seabird-url-plugin/internal"
)

// The X API no longer has a free read tier, so tweet and user data comes from
// FixTweet's public API instead.
const twitterAPIBase = "https://api.fxtwitter.com"

type TwitterProvider struct{}

var (
	twitterPrefix = "[Twitter]"

	// @username
	twitterPrivmsgUserRegex = regexp.MustCompile(`(?:\s|^)@(\w+)`)

	// URL matches
	twitterStatusRegex = regexp.MustCompile(`^/[^/]+/status(?:es)?/(\d+)`)
	twitterUserRegex   = regexp.MustCompile(`^/(\w+)$`)
)

// twitterTweet is the subset of FixTweet's tweet response we care about.
type twitterTweet struct {
	Tweet *struct {
		Text   string `json:"text"`
		Author struct {
			Name       string `json:"name"`
			ScreenName string `json:"screen_name"`
		} `json:"author"`
	} `json:"tweet"`
}

// twitterUser is the subset of FixTweet's user response we care about.
type twitterUser struct {
	User *struct {
		Name        string `json:"name"`
		ScreenName  string `json:"screen_name"`
		Description string `json:"description"`
	} `json:"user"`
}

func NewTwitterProvider() *TwitterProvider {
	return &TwitterProvider{}
}

func (p *TwitterProvider) GetCallbacks() map[string]URLCallback {
	return map[string]URLCallback{
		"twitter.com": p.handle,
		"x.com":       p.handle,
	}
}

func (p *TwitterProvider) GetMessageCallback() MessageCallback {
	// return p.msgCallback
	return nil
}

func (p *TwitterProvider) msgCallback(c *Client, source *pb.ChannelSource, text string) {
	for _, matches := range twitterPrivmsgUserRegex.FindAllStringSubmatch(text, -1) {
		p.getUser(c, source, matches[1])
	}
}

func (p *TwitterProvider) handle(c *Client, source *pb.ChannelSource, u *url.URL) bool {
	if matches := twitterStatusRegex.FindStringSubmatch(u.Path); len(matches) == 2 {
		return p.getTweet(c, source, matches[1])
	} else if matches := twitterUserRegex.FindStringSubmatch(u.Path); len(matches) == 2 {
		return p.getUser(c, source, matches[1])
	}

	return false
}

func (p *TwitterProvider) getUser(c *Client, source *pb.ChannelSource, name string) bool {
	var resp twitterUser

	err := internal.GetJSON(fmt.Sprintf("%s/%s", twitterAPIBase, url.PathEscape(name)), &resp)
	if err != nil || resp.User == nil {
		return false
	}

	// Jay Vana (@jsvana) - Description description
	c.Replyf(source, "%s %s (@%s) - %s",
		twitterPrefix,
		resp.User.Name,
		resp.User.ScreenName,
		twitterCleanText(resp.User.Description))

	return true
}

func (p *TwitterProvider) getTweet(c *Client, source *pb.ChannelSource, id string) bool {
	var resp twitterTweet

	err := internal.GetJSON(fmt.Sprintf("%s/status/%s", twitterAPIBase, id), &resp)
	if err != nil || resp.Tweet == nil {
		return false
	}

	// Tweet text (@jsvana)
	c.Replyf(source, "%s %s (@%s)",
		twitterPrefix,
		twitterCleanText(resp.Tweet.Text),
		resp.Tweet.Author.ScreenName)

	return true
}

// twitterCleanText collapses newlines so multi-line tweets and bios stay on a
// single chat line.
func twitterCleanText(text string) string {
	return strings.TrimSpace(newlineRegex.ReplaceAllLiteralString(text, " "))
}
