package url

import (
	"context"
	"log"
	"net/url"
	"regexp"
	"text/template"

	"github.com/seabird-chat/seabird-go/pb"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/seabird-irc/seabird-url-plugin/internal"
)

var spotifyPrefix = "[Spotify]"

type spotifyMatch struct {
	regex    *regexp.Regexp
	uriRegex *regexp.Regexp
	template *template.Template
	lookup   func(spotify.Client, []string) interface{}
}

var spotifyMatchers = []spotifyMatch{
	{
		regex:    regexp.MustCompile(`^/artist/(.+)$`),
		uriRegex: regexp.MustCompile(`\bspotify:artist:(\w+)\b`),
		template: internal.TemplateMustCompile("spotifyArtist", `{{- .Name -}}`),
		lookup: func(api spotify.Client, matches []string) interface{} {
			artist, err := api.GetArtist(spotify.ID(matches[0]))
			if err != nil {
				log.Printf("Failed to get artist info from Spotify: %s", err)
				return nil
			}
			return artist
		},
	},
	{
		regex:    regexp.MustCompile(`^/album/(.+)$`),
		uriRegex: regexp.MustCompile(`\bspotify:album:(\w+)\b`),
		template: internal.TemplateMustCompile("spotifyAlbum", `
			{{- .Name }} by
			{{- range $index, $element := .Artists }}
			{{- if $index }},{{ end }} {{ $element.Name -}}
			{{- end }} ({{ pluralize .Tracks.Total "track" }})`),
		lookup: func(api spotify.Client, matches []string) interface{} {
			album, err := api.GetAlbum(spotify.ID(matches[0]))
			if err != nil {
				log.Printf("Failed to get album info from Spotify: %s", err)
				return nil
			}
			return album
		},
	},
	{
		regex:    regexp.MustCompile(`^/track/(.+)$`),
		uriRegex: regexp.MustCompile(`\bspotify:track:(\w+)\b`),
		template: internal.TemplateMustCompile("spotifyTrack", `
			"{{ .Name }}" from {{ .Album.Name }} by
			{{- range $index, $element := .Artists }}
			{{- if $index }},{{ end }} {{ $element.Name }}
			{{- end }}`),
		lookup: func(api spotify.Client, matches []string) interface{} {
			track, err := api.GetTrack(spotify.ID(matches[0]))
			if err != nil {
				log.Printf("Failed to get track info from Spotify: %s", err)
				return nil
			}
			return track
		},
	},
	{
		regex:    regexp.MustCompile(`^/playlist/([^/]*)$`),
		uriRegex: regexp.MustCompile(`\bspotify:playlist:(\w+)\b`),
		template: internal.TemplateMustCompile("spotifyPlaylist", `
			"{{- .Name }}" playlist by {{ .Owner.DisplayName }} ({{ pluralize .Tracks.Total "track" }})`),
		lookup: func(api spotify.Client, matches []string) interface{} {
			playlist, err := api.GetPlaylist(spotify.ID(matches[0]))
			if err != nil {
				log.Printf("Failed to get track info from Spotify: %s", err)
				return nil
			}
			return playlist
		},
	},
}

type SpotifyProvider struct {
	client spotify.Client
}

func NewSpotifyProvider(clientID, clientSecret string) (*SpotifyProvider, error) {
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     spotify.TokenURL,
	}

	return &SpotifyProvider{
		client: spotify.NewClient(config.Client(context.Background())),
	}, nil
}

func (p *SpotifyProvider) GetCallbacks() map[string]URLCallback {
	return map[string]URLCallback{
		"open.spotify.com": p.handleURL,
	}
}

func (p *SpotifyProvider) GetMessageCallback() MessageCallback {
	return p.msgCallback
}

func (p *SpotifyProvider) msgCallback(c *Client, event *pb.MessageEvent) {
	for _, matcher := range spotifyMatchers {
		// TODO: handle multiple matches in one message
		if ok := p.handleTarget(matcher, matcher.uriRegex, c, event, event.Text); ok {
			return
		}
	}
}

func (p *SpotifyProvider) handleURL(c *Client, event *pb.MessageEvent, u *url.URL) bool {
	for _, matcher := range spotifyMatchers {
		if p.handleTarget(matcher, matcher.regex, c, event, u.Path) {
			return true
		}
	}

	return false
}

func (p *SpotifyProvider) handleTarget(matcher spotifyMatch, regex *regexp.Regexp, c *Client, event *pb.MessageEvent, target string) bool {
	if !regex.MatchString(target) {
		return false
	}

	matches := regex.FindStringSubmatch(target)
	if len(matches) != 2 {
		return false
	}

	data := matcher.lookup(p.client, matches[1:])
	if data == nil {
		return false
	}

	msg, err := internal.RenderTemplate(matcher.template, spotifyPrefix, data)
	if err != nil {
		log.Printf("Failed to render template: %s", err)
		return false
	}

	c.Reply(event.Source, msg)

	return true
}
