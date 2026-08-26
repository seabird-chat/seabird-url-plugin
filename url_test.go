package url

import (
	"testing"

	seabird "github.com/seabird-chat/seabird-go"
	"github.com/seabird-chat/seabird-go/pb"

	"github.com/stretchr/testify/require"
)

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name     string
		block    *pb.Block
		expected []string
	}{
		{
			name:     "text only",
			block:    seabird.NewTextBlock("look at https://example.com ok"),
			expected: []string{"https://example.com"},
		},
		{
			// A bare URL flattens to "https://example.com
			// (https://example.com)", which used to match twice.
			name: "self-linking url",
			block: seabird.NewLinkBlock(
				"https://example.com",
				seabird.NewTextBlock("https://example.com"),
			),
			expected: []string{"https://example.com"},
		},
		{
			name: "labeled link",
			block: seabird.NewLinkBlock(
				"https://example.com",
				seabird.NewTextBlock("example"),
			),
			expected: []string{"https://example.com"},
		},
		{
			name: "link inside formatting",
			block: seabird.NewContainerBlock(
				seabird.NewTextBlock("see "),
				seabird.NewBoldBlock(seabird.NewLinkBlock(
					"https://example.com",
					seabird.NewTextBlock("example"),
				)),
			),
			expected: []string{"https://example.com"},
		},
		{
			name: "code is ignored",
			block: seabird.NewContainerBlock(
				seabird.NewInlineCodeBlock("https://inline.example.com"),
				seabird.NewFencedCodeBlock("", "https://fenced.example.com"),
			),
			expected: nil,
		},
		{
			name: "text and link",
			block: seabird.NewContainerBlock(
				seabird.NewTextBlock("https://b.example.com "),
				seabird.NewLinkBlock(
					"https://a.example.com",
					seabird.NewTextBlock("a"),
				),
			),
			expected: []string{"https://b.example.com", "https://a.example.com"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, extractURLs(test.block))
		})
	}
}
