package patchwork

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/f2prateek/go-circle"
	"github.com/f2prateek/go-pointers"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Patchwork lets you apply a patch across repos.
type Patchwork struct {
	github *github.Client
	circle circle.CircleCI
}

// New creates a Patchwork client.
func New(githubToken, circleToken string) *Patchwork {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return &Patchwork{
		github: github.NewClient(tc),
		circle: circle.New(circleToken),
	}
}

// Repository is a repository to be patched.
type Repository struct {
	Owner string
	Repo  string
}

// ApplyOptions holds arguments provided to an apply operation.
type ApplyOptions struct {
	Message string
	Branch  string
	Repos   []Repository
}

// Result represents the result of the apply operation.
type Result struct {
	Repo    Repository
	Success bool
}

// Apply the given patch across the given repos.
func (patchwork *Patchwork) Apply(opts ApplyOptions, patch func(repo *github.Repository, directory string)) []Result {
	reposC := make(chan Repository)
	done := make(chan bool)

	var resultsLock sync.Mutex
	results := make([]Result, 0)

	go func() {
		var wg sync.WaitGroup
		for repo := range reposC {
			wg.Add(1)

			go func(repo Repository) {
				defer wg.Done()

				var summary circle.BuildSummary
				for {
					summaries, err := patchwork.circle.RecentBuildsForProject(repo.Owner, repo.Repo)
					if err != nil {
						log.Fatal("couldn't get recent builds for repo", repo, err)
					}

					summary = latestSummary(opts.Branch, summaries)
					if summary.Lifecycle == "finished" {
						break
					}
					time.Sleep(2 * time.Minute)
				}

				if summary.Outcome == "success" {
					pr, _, err := patchwork.github.PullRequests.Create(repo.Owner, repo.Repo, &github.NewPullRequest{
						Title: &opts.Message,
						Head:  &opts.Branch,
						Base:  pointers.String("master"),
					})
					if err != nil {
						log.Fatal("could not create PR", err)
					}

					result, _, err := patchwork.github.PullRequests.Merge(repo.Owner, repo.Repo, *pr.Number, opts.Message)
					if err != nil {
						log.Fatal("could not merge PR", err)
					}
					if !*result.Merged {
						log.Fatal("could not merge PR", err)
					}
					resultsLock.Lock()
					results = append(results, Result{repo, true})
					resultsLock.Unlock()
				} else {
					resultsLock.Lock()
					results = append(results, Result{repo, false})
					resultsLock.Unlock()
				}

			}(repo)
		}
		wg.Wait()
		done <- true
	}()

	for _, repo := range opts.Repos {
		repository, _, err := patchwork.github.Repositories.Get(repo.Owner, repo.Repo)
		if err != nil {
			log.Fatal("could not fetch github information", err)
		}

		dir, err := ioutil.TempDir("", strconv.Itoa(*repository.ID))
		if err != nil {
			log.Fatal("could not create temporary directory", err)
		}
		defer os.Remove(dir)

		patchwork.run(dir, "git", "clone", *repository.SSHURL, dir)
		// Checking out a branch is probably unnecessary.
		patchwork.run(dir, "git", "checkout", "-b", opts.Branch)

		if err := os.Chdir(dir); err != nil {
			log.Fatal("could not change directory", err)
		}

		patch(repository, dir)

		patchwork.run(dir, "git", "add", "-A")
		patchwork.run(dir, "git", "commit", "-m", opts.Message)
		patchwork.run(dir, "git", "push", "origin", opts.Branch)

		reposC <- repo
	}
	close(reposC)

	<-done
	return results
}

func latestSummary(branch string, summaries []circle.BuildSummary) circle.BuildSummary {
	for _, summary := range summaries {
		if summary.Branch == branch {
			return summary
		}
	}
	return circle.BuildSummary{}
}

func (patchwork *Patchwork) run(dir, name string, args ...string) {
	command := exec.Command(name, args...)
	var out bytes.Buffer
	command.Stdout = &out
	command.Stderr = &out
	command.Dir = dir
	if err := command.Run(); err != nil {
		log.Println("could not run", name, args)
		log.Println(out.String())
		log.Fatal(err)
	}
}
