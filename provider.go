package url

import (
	"net/url"

	"github.com/seabird-irc/seabird-url-plugin/pb"
)

// URLCallback is a callback to be registered with the Client. It takes a
// *url.URL representing the found url. It returns true if it was able to handle
// that url and false otherwise.
type URLCallback func(c *Client, event *pb.MessageEvent, u *url.URL) bool

// MessageCallback is a callback to be registered with the Client. It takes an
// event and allows the callback to do what it needs. Note that if any network
// calls need to be made it is recommended to do that in a goroutine.
type MessageCallback func(c *Client, event *pb.MessageEvent)

type Provider interface {
	GetCallbacks() map[string]URLCallback
	GetMessageCallback() MessageCallback
}
