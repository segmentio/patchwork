package patchwork

import (
	"os"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/google/go-github/github"
)

// Options defines how a patch is processed.
type Options struct {
	CommitMessage string
	Skip          bool
}

// Patch is the interface invoked for every github repository to be patched.
type Patch interface {
	// Patches a Github repository cloned locally at the location provided.
	Patch(*github.Repository, string) (Options, error)
}

// The PatchFunc type is an adapter to allow the use of ordinary functions as
// patches. If f is a function with the appropriate signature, PatchFunc(f) is
// a Patch that calls f.
type PatchFunc func(*github.Repository, string) (Options, error)

// Patch calls f(r, d).
func (f PatchFunc) Patch(r *github.Repository, d string) (Options, error) {
	return f(r, d)
}

// Apply the patch to the given repos. Apply clones reach repo locally, invokes
// patch, commits the results to a branch on Github, waits for all checks,
// and merges the branches if *every* check passes.
func Apply(p Patch, repos []*github.Repository) {
	log.SetHandler(cli.New(os.Stdout))
	log.SetLevel(log.DebugLevel)
	run(p, repos)
}
