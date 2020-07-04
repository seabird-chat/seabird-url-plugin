package url

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/seabird-chat/seabird-go/pb"

	"github.com/seabird-irc/seabird-url-plugin/internal"
)

type redditUser struct {
	Data struct {
		Name         string `json:"name"`
		LinkKarma    int    `json:"link_karma"`
		CommentKarma int    `json:"comment_karma"`
		IsGold       bool   `json:"is_gold"`
		IsMod        bool   `json:"is_mod"`
	} `json:"data"`
}

type redditSub struct {
	Data struct {
		URL         string `json:"url"`
		Subscribers int    `json:"subscribers"`
		Description string `json:"public_description"`
		Actives     int    `json:"accounts_active"`
	} `json:"data"`
}

type redditComment struct {
	Data struct {
		Children []struct {
			Data struct {
				Title     string `json:"title"`
				Author    string `json:"author"`
				Score     int    `json:"score"`
				Subreddit string `json:"subreddit"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

var (
	redditPrefix = "[Reddit]"

	// /r/subreddit
	redditPrivmsgSubRegex = regexp.MustCompile(`(?:\s|^)/r/([^\s/]+)`)
	// /u/username
	redditPrivmsgUserRegex = regexp.MustCompile(`(?:\s|^)/(?:u|user)/([^\s/]+)`)

	// URL matches
	redditUserRegex    = regexp.MustCompile(`^/(?:u|user)/([^\s/]+)$`)
	redditCommentRegex = regexp.MustCompile(`^/r/[^/]+/comments/([^/]+)/.*$`)
	redditSubRegex     = regexp.MustCompile(`^/r/([^\s/]+)/?.*$`)
)

type RedditProvider struct{}

func NewRedditProvider() *RedditProvider {
	return &RedditProvider{}
}

func (p *RedditProvider) GetCallbacks() map[string]URLCallback {
	return map[string]URLCallback{
		"reddit.com": redditCallback,
	}
}

func (p *RedditProvider) GetMessageCallback() MessageCallback {
	return redditPrivmsgCallback
}

func redditPrivmsgCallback(c *Client, event *pb.MessageEvent) {
	for _, matches := range redditPrivmsgSubRegex.FindAllStringSubmatch(event.Text, -1) {
		redditGetSub(c, event, matches[1])
	}

	for _, matches := range redditPrivmsgUserRegex.FindAllStringSubmatch(event.Text, -1) {
		redditGetUser(c, event, matches[1])
	}
}

func redditCallback(c *Client, event *pb.MessageEvent, u *url.URL) bool {
	text := u.Path

	//nolint:gocritic
	if matches := redditUserRegex.FindStringSubmatch(text); len(matches) == 2 {
		return redditGetUser(c, event, matches[1])
	} else if matches := redditCommentRegex.FindStringSubmatch(text); len(matches) == 2 {
		return redditGetComment(c, event, matches[1])
	} else if matches := redditSubRegex.FindStringSubmatch(text); len(matches) == 2 {
		return redditGetSub(c, event, matches[1])
	}

	return false
}

func redditGetUser(c *Client, event *pb.MessageEvent, text string) bool {
	ru := &redditUser{}
	if err := internal.GetJSON(fmt.Sprintf("https://www.reddit.com/user/%s/about.json", text), ru); err != nil {
		return false
	}

	// jsvana [gold] has 1 link karma and 1337 comment karma
	gold := ""
	if ru.Data.IsGold {
		gold = " [gold]"
	}

	c.Replyf(event.Source, "%s %s%s has %d link karma and %d comment karma", redditPrefix, ru.Data.Name, gold, ru.Data.LinkKarma, ru.Data.CommentKarma)

	return true
}

func redditGetComment(c *Client, event *pb.MessageEvent, text string) bool {
	rc := []redditComment{}
	if err := internal.GetJSON(fmt.Sprintf("https://www.reddit.com/comments/%s.json", text), rc); err != nil || len(rc) < 1 {
		return false
	}

	cm := rc[0].Data.Children[0].Data

	// Title title - jsvana (/r/vim, score: 5)
	c.Replyf(event.Source, "%s %s - %s (/r/%s, score: %d)", redditPrefix, cm.Title, cm.Author, cm.Subreddit, cm.Score)

	return true
}

func redditGetSub(c *Client, event *pb.MessageEvent, text string) bool {
	rs := &redditSub{}
	if err := internal.GetJSON(fmt.Sprintf("https://www.reddit.com/r/%s/about.json", text), rs); err != nil {
		return false
	}

	// /r/vim - Description description (1 subscriber, 2 actives)
	c.Replyf(event.Source, "%s %s - %s (%s %s, %s %s)",
		redditPrefix,
		rs.Data.URL,
		rs.Data.Description,
		internal.PrettifySuffix(rs.Data.Subscribers),
		internal.PluralizeWord(rs.Data.Subscribers, "subscriber"),
		internal.PrettifySuffix(rs.Data.Actives),
		internal.PluralizeWord(rs.Data.Actives, "active"))

	return true
}
