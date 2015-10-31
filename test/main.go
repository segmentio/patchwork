// Script to update the Circle CI configuration for analytics.js-integrations
// to use Saucelabs in tests.
package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/segmentio/patchwork"
)

func main() {
	p := patchwork.New(os.Getenv("GITHUB_TOKEN"), os.Getenv("CIRCLE_TOKEN"))
	p.Debug()
	p.Patch(func(repo github.Repository, dir string) {
		// sed wasn't playing nicely :(
		circleFile := dir + "/circle.yml"
		circleConfig, err := ioutil.ReadFile(circleFile)
		if err != nil {
			log.Fatal(err)
		}
		lines := strings.Split(string(circleConfig), "\n")
		for i, line := range lines {
			lines[i] = strings.Replace(line, "testing", "testing2", 1)
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(circleFile, []byte(output), 0644)
		if err != nil {
			log.Fatal(err)
		}
	})

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	repo, _, err := client.Repositories.Get("segmentio", "patchwork-test")
	if err != nil {
		log.Fatal(err)
	}
	p.AddRepo(*repo)

	p.Apply()
}
