package url

import (
	"errors"
	"fmt"
	"net/url"

	seabird "github.com/seabird-chat/seabird-go"
	"github.com/seabird-chat/seabird-go/pb"
)

type Client struct {
	*seabird.SeabirdClient

	callbacks        map[string][]URLCallback
	messageCallbacks []MessageCallback
	ignoredBackends  map[string]bool
}

func NewClient(seabirdCoreUrl, seabirdCoreToken string, rawIgnoredBackends []string) (*Client, error) {
	client, err := seabird.NewSeabirdClient(seabirdCoreUrl, seabirdCoreToken)
	if err != nil {
		return nil, err
	}

	ignoredBackends := make(map[string]bool)
	for _, backend := range rawIgnoredBackends {
		ignoredBackends[backend] = true
	}

	return &Client{
		SeabirdClient:   client,
		callbacks:       make(map[string][]URLCallback),
		ignoredBackends: ignoredBackends,
	}, nil
}

func (c *Client) Close() error {
	return c.SeabirdClient.Close()
}

func (c *Client) Register(p Provider) {
	for k, v := range p.GetCallbacks() {
		c.callbacks[k] = append(c.callbacks[k], v)
	}

	if cb := p.GetMessageCallback(); cb != nil {
		c.messageCallbacks = append(c.messageCallbacks, cb)
	}
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

			c.messageCallback(v.Message)
		}
	}

	return errors.New("event stream closed")
}
