package url

import (
	"context"
	"fmt"
	"time"

	"github.com/seabird-irc/seabird-url-plugin/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Client struct {
	identity         *pb.Identity
	grpcChannel      *grpc.ClientConn
	inner            pb.SeabirdClient
	callbacks        map[string][]URLCallback
	messageCallbacks []MessageCallback
}

func NewClient(seabirdCoreUrl, seabirdCoreToken string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	channel, err := grpc.DialContext(ctx, seabirdCoreUrl, grpc.WithTransportCredentials(credentials.NewTLS(nil)), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	return &Client{
		identity: &pb.Identity{AuthMethod: &pb.Identity_Token{
			Token: seabirdCoreToken,
		}},
		grpcChannel: channel,
		inner:       pb.NewSeabirdClient(channel),
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

func (c *Client) ReplyTo(target, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.inner.SendMessage(ctx, &pb.SendMessageRequest{
		Identity: c.identity,
		Target:   target,
		Message:  msg,
	})
	return err
}

func (c *Client) ReplyTof(target, format string, args ...interface{}) error {
	return c.ReplyTo(target, fmt.Sprintf(format, args...))
}

func (c *Client) Run() error {
	events, err := c.inner.StreamEvents(
		context.Background(),
		&pb.StreamEventsRequest{
			Identity: c.identity,
			Commands: map[string]*pb.CommandMetadata{
				"down": {
					Name:      "down",
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
			if v.Command.Command == "down" {
				c.isItDownCallback(v.Command)
			}
		case *pb.Event_Message:
			c.messageCallback(v.Message)
		}
	}
}
