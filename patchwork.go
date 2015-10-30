package patchwork

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/f2prateek/go-circle"
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

// Apply the given patch across the given repos.
func (patchwork *Patchwork) Apply(opts ApplyOptions, patch func(repo *github.Repository, directory string)) {
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

		if err := os.Chdir(dir); err != nil {
			log.Fatal("could not change directory", err)
		}

		patch(repository, dir)

		patchwork.run(dir, "git", "add", "-A")
		patchwork.run(dir, "git", "commit", "-m", opts.Message)
		patchwork.run(dir, "git", "push", "origin", "master:"+opts.Branch)
	}
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
