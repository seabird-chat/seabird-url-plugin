package url

import (
	"context"
	"fmt"
	"time"

	"github.com/seabird-irc/seabird-url-plugin/pb"
	"google.golang.org/grpc"
)

type Client struct {
	grpcChannel      *grpc.ClientConn
	inner            pb.SeabirdClient
	callbacks        map[string][]URLCallback
	messageCallbacks []MessageCallback
}

func NewClient(seabirdCoreUrl, seabirdCoreToken string) (*Client, error) {
	grpcChannel, err := newGRPCClient(seabirdCoreUrl, seabirdCoreToken)
	if err != nil {
		return nil, err
	}

	return &Client{
		grpcChannel: grpcChannel,
		inner:       pb.NewSeabirdClient(grpcChannel),
		callbacks:   make(map[string][]URLCallback),
	}, nil
}

func (c *Client) Close() error {
	return c.grpcChannel.Close()
}

func (c *Client) Register(p Provider) {
	for k, v := range p.GetCallbacks() {
		c.callbacks[k] = append(c.callbacks[k], v)
	}

	if cb := p.GetMessageCallback(); cb != nil {
		c.messageCallbacks = append(c.messageCallbacks, cb)
	}
}

func (c *Client) Reply(source *pb.ChannelSource, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.inner.SendMessage(ctx, &pb.SendMessageRequest{
		ChannelId: source.GetChannelId(),
		Text:      msg,
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
	events, err := c.inner.StreamEvents(
		context.Background(),
		&pb.StreamEventsRequest{
			Commands: map[string]*pb.CommandMetadata{
				"isitdown": {
					Name:      "isitdown",
					ShortHelp: "<website>",
					FullHelp:  "Checks if given website is down",
				},
			},
		},
	)
	if err != nil {
		return err
	}

	for {
		event, err := events.Recv()
		if err != nil {
			return err
		}

		switch v := event.GetInner().(type) {
		case *pb.Event_Command:
			if v.Command.Command == "isitdown" {
				c.isItDownCallback(v.Command)
			}
		case *pb.Event_Message:
			fmt.Printf("%+v\n", v)
			c.messageCallback(v.Message)
		}
	}
}
