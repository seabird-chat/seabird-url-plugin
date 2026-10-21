package url

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/seabird-chat/seabird-go/pb"

	"github.com/seabird-chat/seabird-url-plugin/internal"
)

// Bluesky's public AppView needs no authentication for post and profile reads.
const blueskyAPIBase = "https://public.api.bsky.app/xrpc"

type BlueskyProvider struct{}

var (
	blueskyPrefix = "[Bluesky]"

	// URL matches. The profile segment is either a handle or a DID.
	blueskyPostRegex = regexp.MustCompile(`^/profile/([^/]+)/post/([A-Za-z0-9]+)$`)
	blueskyUserRegex = regexp.MustCompile(`^/profile/([^/]+)$`)
)

// blueskyThread is the subset of getPostThread's response we care about.
type blueskyThread struct {
	Thread struct {
		Post *struct {
			Author blueskyAuthor `json:"author"`
			Record struct {
				Text string `json:"text"`
			} `json:"record"`
		} `json:"post"`
	} `json:"thread"`
}

type blueskyAuthor struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

// blueskyProfile is the subset of getProfile's response we care about.
type blueskyProfile struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

func NewBlueskyProvider() *BlueskyProvider {
	return &BlueskyProvider{}
}

func (p *BlueskyProvider) GetCallbacks() map[string]URLCallback {
	return map[string]URLCallback{
		"bsky.app": p.handle,
	}
}

func (p *BlueskyProvider) GetMessageCallback() MessageCallback {
	return nil
}

func (p *BlueskyProvider) handle(c *Client, source *pb.ChannelSource, u *url.URL) bool {
	if matches := blueskyPostRegex.FindStringSubmatch(u.Path); len(matches) == 3 {
		return p.getPost(c, source, matches[1], matches[2])
	} else if matches := blueskyUserRegex.FindStringSubmatch(u.Path); len(matches) == 2 {
		return p.getUser(c, source, matches[1])
	}

	return false
}

func (p *BlueskyProvider) getUser(c *Client, source *pb.ChannelSource, actor string) bool {
	var resp blueskyProfile

	err := internal.GetJSON(
		fmt.Sprintf("%s/app.bsky.actor.getProfile?actor=%s", blueskyAPIBase, url.QueryEscape(actor)),
		&resp,
	)
	if err != nil || resp.Handle == "" {
		return false
	}

	c.Replyf(source, "%s %s", blueskyPrefix, formatBlueskyProfile(&resp))

	return true
}

func (p *BlueskyProvider) getPost(c *Client, source *pb.ChannelSource, actor, rkey string) bool {
	var resp blueskyThread

	// The AppView resolves handles in the at-uri, which getPosts does not.
	atURI := fmt.Sprintf("at://%s/app.bsky.feed.post/%s", actor, rkey)
	err := internal.GetJSON(
		fmt.Sprintf("%s/app.bsky.feed.getPostThread?depth=0&parentHeight=0&uri=%s",
			blueskyAPIBase, url.QueryEscape(atURI)),
		&resp,
	)
	if err != nil || resp.Thread.Post == nil {
		return false
	}

	c.Replyf(source, "%s %s", blueskyPrefix, formatBlueskyPost(resp.Thread.Post.Record.Text, resp.Thread.Post.Author))

	return true
}

// formatBlueskyPost renders "text (@handle)".
func formatBlueskyPost(text string, author blueskyAuthor) string {
	return fmt.Sprintf("%s (@%s)", twitterCleanText(text), author.Handle)
}

// formatBlueskyProfile renders "Name (@handle) - description".
func formatBlueskyProfile(p *blueskyProfile) string {
	name := p.DisplayName
	if name == "" {
		name = p.Handle
	}

	out := fmt.Sprintf("%s (@%s)", name, p.Handle)
	if desc := twitterCleanText(p.Description); desc != "" {
		out += " - " + desc
	}

	return out
}
