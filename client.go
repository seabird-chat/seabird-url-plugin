package url

import (
	"context"
	"fmt"
	"net/url"
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

	u, err := url.Parse(seabirdCoreUrl)
	if err != nil {
		return nil, err
	}

	// gRPC is a little frustrating in that it doesn't handle actual URLs, so we
	// handle the parsing ourselves.
	host := u.Hostname()
	port := u.Port()
	insecure := false
	if u.Scheme == "http" {
		insecure = true
		if port == "" {
			port = "80"
		}
	} else if u.Scheme == "https" {
		if port == "" {
			port = "443"
		}
	} else {
		return nil, fmt.Errorf("Unknown scheme: %s", u.Scheme)
	}

	// If connecting over http, we need to allow insecure connections or it will
	// not work.
	var opts []grpc.DialOption
	if insecure {
		opts = append(opts, grpc.WithInsecure(), grpc.WithBlock())
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(nil)), grpc.WithBlock())
	}

	finalUrl := fmt.Sprintf("%s:%s", host, port)

	channel, err := grpc.DialContext(ctx, finalUrl, opts...)
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
			c.messageCallback(v.Message)
		}
	}
}
