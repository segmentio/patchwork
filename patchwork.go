package patchwork

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Patchwork lets you apply a patch across repos.
type Patchwork struct {
	github *github.Client
}

// New creates a Patchwork client.
func New(token string) *Patchwork {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)
	return &Patchwork{
		github: client,
	}
}

// Repository is a repository to be patched.
type Repository struct {
	Owner string
	Repo  string
}

// ApplyOptions represents the arguments that can be passed to Apply.
type ApplyOptions struct {
	Repos   []Repository
	Message string
	Branch  string
}

// AddRepo adds a repo to apply the patch.
func (options *ApplyOptions) AddRepo(owner, repo string) {
	if len(options.Repos) == 0 {
		options.Repos = make([]Repository, 0)
	}
	options.Repos = append(options.Repos, Repository{owner, repo})
}

// Apply the given patch across the given repos.
func (patchwork *Patchwork) Apply(options ApplyOptions, patch func(repo *github.Repository, directory string)) {
	for _, repo := range options.Repos {
		repository, _, err := patchwork.github.Repositories.Get(repo.Owner, repo.Repo)
		if err != nil {
			log.Fatal("could not fetch github information", err)
		}

		dir, err := ioutil.TempDir("", strconv.Itoa(*repository.ID))
		if err != nil {
			log.Fatal("could not create temporary directory", err)
		}
		defer os.Remove(dir)

		if err := exec.Command("git", "clone", *repository.SSHURL, dir).Run(); err != nil {
			log.Fatal("could not clone directory", err)
		}

		if err := os.Chdir(dir); err != nil {
			log.Fatal("could not change directory", err)
		}

		patch(repository, dir)

		out, err := exec.Command("git", "diff").Output()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(out))

		if err := exec.Command("git", "add", "-A").Run(); err != nil {
			log.Fatal("could not run git add -A", err)
		}

		if err := exec.Command("git", "commit", "-m", options.Message).Run(); err != nil {
			log.Fatal("could not commit files", err)
		}

		if err := exec.Command("git", "push", "origin", "master:"+options.Branch).Run(); err != nil {
			log.Fatal("could not push", err)
		}
	}
}
