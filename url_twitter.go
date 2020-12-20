package url

import (
	"net/url"
	"regexp"
	"strconv"

	"github.com/ChimeraCoder/anaconda"
	"github.com/seabird-chat/seabird-go/pb"
)

type TwitterProvider struct {
	api *anaconda.TwitterApi
}

var (
	twitterPrefix = "[Twitter]"

	// @username
	twitterPrivmsgUserRegex = regexp.MustCompile(`(?:\s|^)@(\w+)`)

	// URL matches
	twitterStatusRegex = regexp.MustCompile(`^/.*?/status/(.+)$`)
	twitterUserRegex   = regexp.MustCompile(`^/([^/]+)$`)
)

func NewTwitterProvider(consumerKey, consumerSecret, accessToken, accessTokenSecret string) *TwitterProvider {
	anaconda.SetConsumerKey(consumerKey)
	anaconda.SetConsumerSecret(consumerSecret)

	return &TwitterProvider{
		api: anaconda.NewTwitterApi(accessToken, accessTokenSecret),
	}
}

func (p *TwitterProvider) GetCallbacks() map[string]URLCallback {
	return map[string]URLCallback{
		"twitter.com": p.handle,
	}
}

func (p *TwitterProvider) GetMessageCallback() MessageCallback {
	// return p.msgCallback
	return nil
}

func (p *TwitterProvider) msgCallback(c *Client, event *pb.MessageEvent) {
	for _, matches := range twitterPrivmsgUserRegex.FindAllStringSubmatch(event.Text, -1) {
		p.getUser(c, event, matches[1])
	}
}

func (p *TwitterProvider) handle(c *Client, event *pb.MessageEvent, u *url.URL) bool {
	if matches := twitterUserRegex.FindStringSubmatch(u.Path); len(matches) == 2 {
		return p.getUser(c, event, matches[1])
	} else if matches := twitterStatusRegex.FindStringSubmatch(u.Path); len(matches) == 2 {
		return p.getTweet(c, event, matches[1])
	}

	return false
}

func (p *TwitterProvider) getUser(c *Client, event *pb.MessageEvent, text string) bool {
	user, err := p.api.GetUsersShow(text, nil)
	if err != nil {
		return false
	}

	// Jay Vana (@jsvana) - Description description
	c.Replyf(event.Source, "%s %s (@%s) - %s", twitterPrefix, user.Name, user.ScreenName, user.Description)

	return true
}

func (p *TwitterProvider) getTweet(c *Client, event *pb.MessageEvent, text string) bool {
	id, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return false
	}

	tweet, err := p.api.GetTweet(id, nil)
	if err != nil {
		return false
	}

	// Tweet text (@jsvana)
	c.Replyf(event.Source, "%s %s (@%s)", twitterPrefix, tweet.Text, tweet.User.ScreenName)

	return true
}
