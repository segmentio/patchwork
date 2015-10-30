// Script to update the Circle CI configuration for analytics.js-integrations
// to use Saucelabs in tests.
package main

import (
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/segmentio/patchwork"
)

func main() {
	p := patchwork.New(os.Getenv("GITHUB_TOKEN"), os.Getenv("CIRCLE_TOKEN"))
	p.Debug = true

	rand.Seed(time.Now().Unix())
	n := strconv.Itoa(rand.Intn(4000))

	opts := &patchwork.ApplyOptions{}
	opts.Message = "Testing Patchwork!" + n
	opts.Branch = "test" + n
	opts.Repos = []patchwork.Repository{
		{"segmentio", "patchwork-test"},
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
			lines[i] = strings.Replace(line, "testing", "testing success", 1)
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(circleFile, []byte(output), 0644)
		if err != nil {
			log.Fatalln(err)
		}
	})
}
