package patchwork

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/apex/log"

	"github.com/google/go-github/github"
)

// commit represents
type commit struct {
	repo *github.Repository
	sha  string
}

func run(p Patch, repos []*github.Repository) {
	commits := applyPatchesLocally(p, repos)

	for commit := range commits {
		fmt.Println(commit)
	}

	// todo: implement https://godoc.org/github.com/google/go-github/github#RepositoriesService.GetCombinedStatus
}

// Clones repos locally, checks out a branch, applies a patch, commits and publishes the result.
// Commits are published on the channel returned.
func applyPatchesLocally(p Patch, repos []*github.Repository) <-chan commit {
	out := make(chan commit, len(repos))

	go func() {
		defer close(out)

		patchID := strconv.Itoa(rand.Int())
		branch := "patch-" + patchID
		genericCommitMessage := "Applying patch " + patchID

		for _, repo := range repos {
			ctx := log.WithField("slug", *repo.FullName)

			dir, err := ioutil.TempDir("", strconv.Itoa(*repo.ID))
			if err != nil {
				ctx.WithError(err).Fatal("could not create temporary directory")
			}
			defer os.Remove(dir)

			ctx.Debugf("cloning %s", *repo.SSHURL)
			execCommand(ctx, dir, "git", "clone", *repo.SSHURL, dir)
			execCommand(ctx, dir, "git", "checkout", "-b", branch)

			// Change the directory for the patch handler.
			if err := os.Chdir(dir); err != nil {
				ctx.WithError(err).Fatalf("could not change directory to %s", dir)
			}

			ctx.Debug("patching")
			// todo: defer recover since we're running 3rd party code.
			opts, err := p.Patch(repo, dir)
			if err != nil {
				ctx.WithError(err).Fatalf("could not change apply patch")
			}
			commitMessage := opts.CommitMessage
			if commitMessage == "" {
				commitMessage = genericCommitMessage
			}

			ctx.Debug("publishing patch")
			execCommand(ctx, dir, "git", "add", "-A")
			execCommand(ctx, dir, "git", "commit", "-m", "\""+commitMessage+"\"")
			execCommand(ctx, dir, "git", "push", "origin", branch)
			sha := strings.Trim(execCommand(ctx, dir, "git", "rev-parse", "HEAD"), "\n ")

			out <- commit{repo, sha}
		}
	}()

	return out
}

// execCommand runs command `name` in the given `dir` directory with the given
// arguments. It logs the output of the command in case of a failure.
func execCommand(ctx *log.Entry, dir, name string, args ...string) string {
	command := exec.Command(name, args...)
	command.Dir = dir

	out, err := command.CombinedOutput()
	if err != nil {
		ctx.WithError(err).WithFields(&log.Fields{
			"command": name,
			"args":    args,
			"output":  string(out),
		}).Fatal("could not run command")
	}

	return string(out)
}
