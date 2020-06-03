package url

import (
	"context"
	"errors"
	"log"
	"net/url"
	"regexp"
	"strconv"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"

	"github.com/seabird-irc/seabird-url-plugin/internal"
	"github.com/seabird-irc/seabird-url-plugin/pb"
)

type GithubProvider struct {
	api *github.Client
}

func NewGithubProvider(token string) *GithubProvider {
	// Create an oauth2 client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.TODO(), ts)

	// Create a github client from the oauth2 client
	return &GithubProvider{
		api: github.NewClient(tc),
	}
}

func (p *GithubProvider) GetCallbacks() map[string]URLCallback {
	return map[string]URLCallback{
		"github.com":      p.githubCallback,
		"gist.github.com": p.gistCallback,
	}
}

func (p *GithubProvider) GetMessageCallback() MessageCallback {
	return nil
}

var (
	githubUserRegex  = regexp.MustCompile(`^/([^/]+)$`)
	githubRepoRegex  = regexp.MustCompile(`^/([^/]+)/([^/]+)$`)
	githubIssueRegex = regexp.MustCompile(`^/([^/]+)/([^/]+)/issues/([^/]+)$`)
	githubPullRegex  = regexp.MustCompile(`^/([^/]+)/([^/]+)/pull/([^/]+)$`)
	githubGistRegex  = regexp.MustCompile(`^/([^/]+)/([^/]+)$`)

	githubPrefix = "[Github]"
)

func parseUserRepoNum(matches []string) (string, string, int, error) {
	if len(matches) != 4 {
		return "", "", 0, errors.New("Incorrect number of matches")
	}

	retInt, err := strconv.ParseInt(matches[3], 10, 32)
	if err != nil {
		return "", "", 0, err
	}

	return matches[1], matches[2], int(retInt), nil
}

func (p *GithubProvider) githubCallback(c *Client, event *pb.MessageEvent, u *url.URL) bool {
	//nolint:gocritic
	if githubUserRegex.MatchString(u.Path) {
		return p.getUser(c, event, u.Path)
	} else if githubRepoRegex.MatchString(u.Path) {
		return p.getRepo(c, event, u.Path)
	} else if githubIssueRegex.MatchString(u.Path) {
		return p.getIssue(c, event, u.Path)
	} else if githubPullRegex.MatchString(u.Path) {
		return p.getPull(c, event, u.Path)
	}

	return false
}

func (p *GithubProvider) gistCallback(c *Client, event *pb.MessageEvent, u *url.URL) bool {
	if githubGistRegex.MatchString(u.Path) {
		return p.getGist(c, event, u.Path)
	}

	return false
}

// Jay Vana (@jsvana) at Facebook - Bio bio bio
var userTemplate = internal.TemplateMustCompile("githubUser", `
{{- if .user.Name -}}
{{ .user.Name }}
{{- with .user.Login }}(@{{ . }}){{ end -}}
{{- else if .user.Login -}}
@{{ .user.Login }}
{{- end -}}
{{- with .user.Company }} at {{ . }}{{ end -}}
{{- with .user.Bio }} - {{ . }}{{ end -}}
`)

func (p *GithubProvider) getUser(c *Client, event *pb.MessageEvent, url string) bool {
	matches := githubUserRegex.FindStringSubmatch(url)
	if len(matches) != 2 {
		return false
	}

	user, _, err := p.api.Users.Get(context.TODO(), matches[1])
	if err != nil {
		log.Printf("Failed to get user from github: %s", err)
		return false
	}

	ret, err := internal.RenderTemplate(
		userTemplate, githubPrefix,
		map[string]interface{}{
			"user": user,
		},
	)
	if err != nil {
		log.Printf("Failed to render template: %s", err)
		return false
	}

	c.ReplyTo(event.ReplyTo, ret)

	return true

}

// jsvana/alfred [PHP] (forked from belak/alfred) Last pushed to 2 Jan 2015 - Description, 1 fork, 2 open issues, 4 stars
var repoTemplate = internal.TemplateMustCompile("githubRepo", `
{{- .repo.FullName -}}
{{- with .repo.Language }} [{{ . }}]{{ end -}}
{{- if and .repo.Fork .repo.Parent }} (forked from {{ .repo.Parent.FullName }}){{ end }}
{{- with .repo.PushedAt }} Last pushed to {{ . | dateFormat "2 Jan 2006" }}{{ end }}
{{- with .repo.Description }} - {{ . }}{{ end }}
{{- with .repo.ForksCount }}, {{ prettifySuffix . }} {{ pluralizeWord . "fork" }}{{ end }}
{{- with .repo.OpenIssuesCount }}, {{ prettifySuffix . }} {{ pluralizeWord . "open issue" }}{{ end }}
{{- with .repo.StargazersCount }}, {{ prettifySuffix . }} {{ pluralizeWord . "star" }}{{ end }}
`)

func (p *GithubProvider) getRepo(c *Client, event *pb.MessageEvent, url string) bool {
	matches := githubRepoRegex.FindStringSubmatch(url)
	if len(matches) != 3 {
		return false
	}

	user := matches[1]
	repoName := matches[2]
	repo, _, err := p.api.Repositories.Get(context.TODO(), user, repoName)

	if err != nil {
		log.Printf("Failed to get repo from github: %s", err)
		return false
	}

	// If the repo doesn't have a name, we get outta there
	if repo.FullName == nil || *repo.FullName == "" {
		log.Println("Invalid repo returned from github")
		return false
	}

	ret, err := internal.RenderTemplate(
		repoTemplate, githubPrefix,
		map[string]interface{}{
			"repo": repo,
		},
	)
	if err != nil {
		log.Printf("Failed to render template: %s", err)
		return false
	}

	c.ReplyTo(event.ReplyTo, ret)

	return true

}

// Issue #42 on belak/go-seabird [open] (assigned to jsvana) - Issue title [created 2 Jan 2015]
var issueTemplate = internal.TemplateMustCompile("githubIssue", `
Issue #{{ .issue.Number }} on {{ .user }}/{{ .repo }} [{{ .issue.State }}]
{{- with .issue.Assignee }} (assigned to {{ .Login }}){{ end }}
{{- with .issue.Title }} - {{ . }}{{ end }}
{{- with .issue.CreatedAt }} [created {{ . | dateFormat "2 Jan 2006" }}]{{ end }}
`)

func (p *GithubProvider) getIssue(c *Client, event *pb.MessageEvent, url string) bool {
	matches := githubIssueRegex.FindStringSubmatch(url)

	user, repo, issueNum, err := parseUserRepoNum(matches)
	if err != nil {
		log.Printf("Failed to parse URL: %s", err)
		return false
	}

	issue, _, err := p.api.Issues.Get(context.TODO(), user, repo, issueNum)
	if err != nil {
		log.Printf("Failed to get issue from github: %s", err)
		return false
	}

	ret, err := internal.RenderTemplate(
		issueTemplate, githubPrefix,
		map[string]interface{}{
			"issue": issue,
			"user":  user,
			"repo":  repo,
		},
	)
	if err != nil {
		log.Printf("Failed to render template: %s", err)
		return false
	}

	c.ReplyTo(event.ReplyTo, ret)

	return true

}

// Pull request #59 on belak/go-seabird [open] - Title title title [created 4 Jan 2015], 1 commit, 4 comments, 2 changed files
var prTemplate = internal.TemplateMustCompile("githubPRTemplate", `
Pull request #{{ .pull.Number }} on {{ .user }}/{{ .repo }} [{{ .pull.State }}]
{{- with .pull.User.Login }} created by {{ . }}{{ end }}
{{- with .pull.Title }} - {{ . }}{{ end }}
{{- with .pull.CreatedAt }} [created {{ . | dateFormat "2 Jan 2006" }}]{{ end }}
{{- with .pull.Commits }}, {{ pluralize . "commit" }}{{ end }}
{{- with .pull.Comments }}, {{ pluralize . "comment" }}{{ end }}
{{- with .pull.ChangedFiles }}, {{ pluralize . "changed file" }}{{ end }}
`)

func (p *GithubProvider) getPull(c *Client, event *pb.MessageEvent, url string) bool {
	matches := githubPullRegex.FindStringSubmatch(url)

	user, repo, pullNum, err := parseUserRepoNum(matches)
	if err != nil {
		log.Printf("Failed to parse URL: %s", err)
		return false
	}

	pull, _, err := p.api.PullRequests.Get(context.TODO(), user, repo, pullNum)
	if err != nil {
		log.Printf("Failed to get pr from github: %s", err)
		return false
	}

	ret, err := internal.RenderTemplate(
		prTemplate, githubPrefix,
		map[string]interface{}{
			"user": user,
			"repo": repo,
			"pull": pull,
		},
	)
	if err != nil {
		log.Printf("Failed to render template: %s", err)
		return false
	}

	c.ReplyTo(event.ReplyTo, ret)

	return true

}

// Created 3 Jan 2015 by belak - Description description, 1 file, 3 comments
var gistTemplate = internal.TemplateMustCompile("gist", `
Created {{ .gist.CreatedAt | dateFormat "2 Jan 2006" }}
{{- with .gist.Owner.Login }} by {{ . }}{{ end }}
{{- with .gist.Description }} - {{ . }}{{ end }}
{{- with .gist.Comments }}, {{ pluralize . "comment" }}{{ end }}
`)

func (p *GithubProvider) getGist(c *Client, event *pb.MessageEvent, url string) bool {
	matches := githubGistRegex.FindStringSubmatch(url)
	if len(matches) != 3 {
		return false
	}

	id := matches[2]

	gist, _, err := p.api.Gists.Get(context.TODO(), id)
	if err != nil {
		log.Printf("Failed to get gist: %s", err)
		return false
	}

	ret, err := internal.RenderTemplate(
		gistTemplate, githubPrefix,
		map[string]interface{}{
			"gist": gist,
		},
	)
	if err != nil {
		log.Printf("Failed to render template: %s", err)
		return false
	}

	c.ReplyTo(event.ReplyTo, ret)

	return true
}
