package url

import (
	"net/url"

	"github.com/seabird-irc/seabird-url-plugin/pb"
)

func (c *Client) isItDownCallback(event *pb.CommandEvent) {
	go func() {
		url, err := url.Parse(event.Arg)
		if err != nil {
			c.ReplyTof(event.ReplyTo, "%s: URL doesn't appear to be valid", event.Sender)
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
			c.ReplyTof(event.ReplyTo, "%s: It's not just you! %s looks down from here.", event.Sender, url)
			return
		}

		c.ReplyTof(event.ReplyTo, "%s: It's just you! %s looks up from here!", event.Sender, url)
	}()
}
