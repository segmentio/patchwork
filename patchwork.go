package patchwork

import (
	"bytes"
	"fmt"
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
	Debug  bool
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

// Apply the given patch across the given repos.
func (patchwork *Patchwork) Apply(opts ApplyOptions, patch func(repo *github.Repository, directory string)) {
	reposC := make(chan Repository)
	doneBuilds := make(chan bool)

	var resultsLock sync.Mutex
	var results []circle.BuildSummary

	go func() {
		var wg sync.WaitGroup
		for repo := range reposC {
			wg.Add(1)

			go func(repo Repository) {
				defer wg.Done()

				for {
					patchwork.logf("waiting for CI of branch %v of %v", opts.Branch, repo)
					time.Sleep(1 * time.Minute)

					patchwork.logf("fetching CI status for branch %v of %v", opts.Branch, repo)
					summaries, err := patchwork.circle.RecentBuildsForProjectBranch(repo.Owner, repo.Repo, opts.Branch, circle.RecentBuildsOptions{
						Filter: pointers.String("completed"),
					})
					if err != nil {
						log.Fatal("couldn't get recent builds for repo", repo, err)
					}

					if len(summaries) == 0 {
						patchwork.logf("no completed builds for branch %v of repo %v", opts.Branch, repo)
						continue
					}

					patchwork.logf("successfully built branch %v of repo %v", opts.Branch, repo)
					resultsLock.Lock()
					results = append(results, summaries[0])
					resultsLock.Unlock()
					break
				}
			}(repo)
		}
		wg.Wait()
		doneBuilds <- true
	}()

	for _, repo := range opts.Repos {
		patchwork.logf("fetching github information for %v", repo)
		repository, _, err := patchwork.github.Repositories.Get(repo.Owner, repo.Repo)
		if err != nil {
			log.Fatal("could not fetch github information", err)
		}

		patchwork.logf("creating temp directory for %v", repo)
		dir, err := ioutil.TempDir("", strconv.Itoa(*repository.ID))
		if err != nil {
			log.Fatal("could not create temporary directory", err)
		}
		defer os.Remove(dir)

		patchwork.logf("cloning %v", repo)
		run(dir, "git", "clone", *repository.SSHURL, dir)
		// Checking out a branch is probably unnecessary.
		patchwork.logf("checking out branch %v for %v", opts.Branch, repo)
		run(dir, "git", "checkout", "-b", opts.Branch)

		if err := os.Chdir(dir); err != nil {
			log.Fatal("could not change directory", err)
		}

		patch(repository, dir)

		patchwork.logf("pushing changes to branch %v for %v", opts.Branch, repo)
		run(dir, "git", "add", "-A")
		run(dir, "git", "commit", "-m", opts.Message)
		run(dir, "git", "push", "origin", opts.Branch)

		reposC <- repo
	}
	close(reposC)

	<-doneBuilds

	success := true
	for _, result := range results {
		if result.Outcome != "success" {
			success = false
			fmt.Printf("repo %s/%s failed to build\n", result.Username, result.Reponame)
		}
	}

	if !success {
		log.Fatal("There were some build failures. Aborting.")
	}

	for _, result := range results {
		pr, _, err := patchwork.github.PullRequests.Create(result.Username, result.Reponame, &github.NewPullRequest{
			Title: &opts.Message,
			Head:  &opts.Branch,
			Base:  pointers.String("master"),
		})
		if err != nil {
			log.Fatal("could not create PR", err)
		}

		result, _, err := patchwork.github.PullRequests.Merge(result.Username, result.Reponame, *pr.Number, opts.Message)
		if err != nil {
			log.Fatal("could not merge PR", err)
		}
		if !*result.Merged {
			log.Fatal("could not merge PR", err)
		}
	}
}

func (patchwork *Patchwork) logf(format string, v ...interface{}) {
	if patchwork.Debug {
		log.Printf(format, v...)
	}
}

// Run will run command `name` in the given `dir` directory with the given
// arguments. It also logs the output of the command in case of a failure.
func run(dir, name string, args ...string) {
	command := exec.Command(name, args...)
	var buf bytes.Buffer
	command.Stdout = &buf
	command.Stderr = &buf
	command.Dir = dir
	if err := command.Run(); err != nil {
		log.Println("could not run", name, args)
		log.Println(buf.String())
		log.Fatal(err)
	}
}
