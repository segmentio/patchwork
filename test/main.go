package main

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/google/go-github/github"
	"github.com/segmentio/patchwork"
	"golang.org/x/oauth2"
)

func main() {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	repo, _, err := client.Repositories.Get("segmentio", "patchwork-test")
	if err != nil {
		log.WithError(err).Fatal("could not fetch repo")
	}

	patchwork.Apply(patchwork.PatchFunc(patch), []*github.Repository{repo})
}

func patch(repo *github.Repository, dir string) (patchwork.Options, error) {
	circleFile := dir + "/circle.yml"
	circleConfig, err := ioutil.ReadFile(circleFile)
	if err != nil {
		log.WithError(err).Fatal("could not read circle.yml")
	}
	lines := strings.Split(string(circleConfig), "\n")
	for i, line := range lines {
		lines[i] = strings.Replace(line, "testing", "testing2", 1)
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(circleFile, []byte(output), 0644)
	if err != nil {
		log.WithError(err).Fatal("could not write to circle.yml")
	}

	return patchwork.Options{}, nil
}
