package url

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	seabird "github.com/seabird-chat/seabird-go"
	"github.com/seabird-chat/seabird-go/pb"
)

type Client struct {
	*seabird.Client

	callbacks        map[string][]URLCallback
	messageCallbacks []MessageCallback
	ignoredBackends  map[string]bool
}

func NewClient(seabirdCoreUrl, seabirdCoreToken string, rawIgnoredBackends []string) (*Client, error) {
	client, err := seabird.NewClient(seabirdCoreUrl, seabirdCoreToken)
	if err != nil {
		return nil, err
	}

	ignoredBackends := make(map[string]bool)
	for _, backend := range rawIgnoredBackends {
		ignoredBackends[backend] = true
	}

	return &Client{
		Client:          client,
		callbacks:       make(map[string][]URLCallback),
		ignoredBackends: ignoredBackends,
	}, nil
}

func (c *Client) Register(p Provider) {
	for k, v := range p.GetCallbacks() {
		c.callbacks[k] = append(c.callbacks[k], v)
	}

	if cb := p.GetMessageCallback(); cb != nil {
		c.messageCallbacks = append(c.messageCallbacks, cb)
	}
}

// TODO: currently Reply in seabird-go doesn't expose tags, so we copy all the
// Reply variants here and make sure to set the proper tags.
func (c *Client) Reply(source *pb.ChannelSource, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Inner.SendMessage(ctx, &pb.SendMessageRequest{
		ChannelId: source.GetChannelId(),
		Text:      msg,
		Tags: map[string]string{
			"proxy/skip":         "1",
			"proxy/internal-tag": "1",
		},
	})

	return err
}

func (c *Client) Replyf(source *pb.ChannelSource, format string, args ...interface{}) error {
	return c.Reply(source, fmt.Sprintf(format, args...))
}

func (c *Client) MentionReply(source *pb.ChannelSource, msg string) error {
	return c.Reply(source, fmt.Sprintf("%s: %s", source.GetUser().GetDisplayName(), msg))
}

func (c *Client) MentionReplyf(source *pb.ChannelSource, format string, args ...interface{}) error {
	return c.MentionReply(source, fmt.Sprintf(format, args...))
}

func (c *Client) Run() error {
	events, err := c.StreamEvents(map[string]*pb.CommandMetadata{
		"isitdown": {
			Name:      "isitdown",
			ShortHelp: "<website>",
			FullHelp:  "Checks if given website is down",
		},
	})
	if err != nil {
		return err
	}
	defer events.Close()

	for event := range events.C {
		// Skip any events we sent
		if event.Tags["proxy/internal-tag"] == "1" {
			continue
		}

		// Skip any events others asked to be skipped
		if event.Tags["url/skip"] == "1" {
			continue
		}

		switch v := event.GetInner().(type) {
		case *pb.Event_Command:
			if v.Command.Command == "isitdown" {
				c.isItDownCallback(v.Command)
			}
		case *pb.Event_Message:
			fmt.Printf("%+v\n", v)
			id, err := url.Parse(v.Message.Source.ChannelId)
			if err != nil {
				fmt.Printf("failed to parse channel id %q: %s\n", v.Message.Source.ChannelId, err)
				continue
			}

			if c.ignoredBackends[id.Scheme] {
				fmt.Printf("message refers to ignored backend %s\n", id.Scheme)
				continue
			}

			c.messageCallback(v.Message.Source, v.Message.Text)
		case *pb.Event_SendMessage:
			fmt.Printf("%+v\n", v)
			id, err := url.Parse(v.SendMessage.ChannelId)
			if err != nil {
				fmt.Printf("failed to parse channel id %q: %s\n", v.SendMessage.ChannelId, err)
				continue
			}

			if c.ignoredBackends[id.Scheme] {
				fmt.Printf("message refers to ignored backend %s\n", id.Scheme)
				continue
			}

			// We construct a bogus ChannelSource here to make the interface
			// simpler. Thankfully, we only use .Reply/.Replyf so we only need
			// the channelId here.
			c.messageCallback(&pb.ChannelSource{
				ChannelId: v.SendMessage.ChannelId,
			}, v.SendMessage.Text)
		}
	}

	return errors.New("event stream closed")
}
