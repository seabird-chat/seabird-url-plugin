package url

import (
	"net/url"

	"github.com/seabird-chat/seabird-go/pb"
)

// URLCallback is a callback to be registered with the Client. It takes a
// *url.URL representing the found url. It returns true if it was able to handle
// that url and false otherwise.
type URLCallback func(c *Client, source *pb.ChannelSource, u *url.URL) bool

// MessageCallback is a callback to be registered with the Client. It takes an
// event and allows the callback to do what it needs. Note that if any network
// calls need to be made it is recommended to do that in a goroutine.
type MessageCallback func(c *Client, source *pb.ChannelSource, text string)

type Provider interface {
	GetCallbacks() map[string]URLCallback
	GetMessageCallback() MessageCallback
}
