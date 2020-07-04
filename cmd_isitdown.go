package url

import (
	"net/url"

	"github.com/seabird-chat/seabird-go/pb"
)

func (c *Client) isItDownCallback(event *pb.CommandEvent) {
	go func() {
		url, err := url.Parse(event.Arg)
		if err != nil {
			c.MentionReply(event.Source, "URL doesn't appear to be valid")
			return
		}

		if url.Scheme == "" {
			url.Scheme = "http"
		}

		resp, err := client.Head(url.String())
		if err == nil {
			defer resp.Body.Close()
		}

		if err != nil || resp.StatusCode != 200 {
			c.MentionReplyf(event.Source, "It's not just you! %s looks down from here.", url)
			return
		}

		c.MentionReplyf(event.Source, "It's just you! %s looks up from here!", url)
	}()
}
