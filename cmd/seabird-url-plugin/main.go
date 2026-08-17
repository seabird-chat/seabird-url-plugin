package main

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"

	url "github.com/seabird-irc/seabird-url-plugin"
)

func main() {
	_ = godotenv.Load()

	coreURL := os.Getenv("SEABIRD_HOST")
	coreToken := os.Getenv("SEABIRD_TOKEN")

	if coreURL == "" || coreToken == "" {
		log.Fatal("Missing SEABIRD_HOST or SEABIRD_TOKEN")
	}

	var ignoredBackends []string
	rawIgnoredBackends := os.Getenv("IGNORED_BACKENDS")
	if rawIgnoredBackends != "" {
		ignoredBackends = strings.Split(rawIgnoredBackends, ",")
	}

	c, err := url.NewClient(
		coreURL,
		coreToken,
		ignoredBackends,
	)
	if err != nil {
		log.Fatal(err)
	}

	registerProviders(c)

	err = c.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func registerProviders(c *url.Client) {
	var err error
	var provider url.Provider = url.NewBitbucketProvider()
	c.Register(provider)

	if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
		provider = url.NewGithubProvider(githubToken)
		c.Register(provider)
	} else {
		log.Fatal("Missing GITHUB_TOKEN")
	}

	provider = url.NewRedditProvider()
	c.Register(provider)

	spotifyClientID := os.Getenv("SPOTIFY_CLIENT_ID")
	spotifyClientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if spotifyClientID == "" || spotifyClientSecret == "" {
		log.Fatal("Missing SPOTIFY_CLIENT_ID or SPOTIFY_CLIENT_SECRET")
	}
	provider, err = url.NewSpotifyProvider(spotifyClientID, spotifyClientSecret)
	if err != nil {
		log.Fatalf("Failed to connect to Spotify: %s", err)
	}
	c.Register(provider)

	provider = url.NewTwitterProvider()
	c.Register(provider)

	provider = url.NewXKCDProvider()
	c.Register(provider)

	if youtubeToken := os.Getenv("YOUTUBE_TOKEN"); youtubeToken != "" {
		provider = url.NewYoutubeProvider(youtubeToken)
		c.Register(provider)
	} else {
		log.Fatal("Missing YOUTUBE_TOKEN")
	}
}
