// Script to update the Circle CI configuration for analytics.js-integrations
// to use Saucelabs in tests.
package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"github.com/segmentio/patchwork"
)

func main() {
	p := patchwork.New(os.Getenv("GITHUB_TOKEN"), os.Getenv("CIRCLE_TOKEN"))

	opts := &patchwork.ApplyOptions{}
	opts.Message = "Updating Circle config to use Saucelabs."
	opts.Branch = "make-test-sauce"
	opts.Repos = []patchwork.Repository{
		{"segment-integrations", "analytics.js-integration-chameleon"},
	}

	p.Apply(*opts, func(repo *github.Repository, dir string) {
		// sed wasn't playing nicely :(
		circleFile := dir + "/circle.yml"
		circleConfig, err := ioutil.ReadFile(circleFile)
		if err != nil {
			log.Fatal(err)
		}
		lines := strings.Split(string(circleConfig), "\n")
		for i, line := range lines {
			lines[i] = strings.Replace(line, "make test", "make test-sauce", 1)
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(circleFile, []byte(output), 0644)
		if err != nil {
			log.Fatalln(err)
		}
	})
}
