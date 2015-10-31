package patchwork

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/f2prateek/go-circle"
	"github.com/f2prateek/go-pointers"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Patchwork lets you apply a patch across repos.
type Patchwork struct {
	github    *github.Client
	circle    circle.CircleCI
	debug     bool
	patch     func(repo github.Repository, directory string)
	repos     []github.Repository
	branch    string
	commitMsg string
	duration  time.Duration
}

// New creates a Patchwork client.
func New(githubToken, circleToken string) *Patchwork {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	rand.Seed(time.Now().Unix())
	id := strconv.Itoa(rand.Int())

	return &Patchwork{
		github:    github.NewClient(tc),
		circle:    circle.New(circleToken),
		repos:     make([]github.Repository, 0),
		branch:    "patch-" + id,
		commitMsg: "Applying patch " + id,
		duration:  time.Second * 360,
	}
}

// Debug enables debugging output.
func (patchwork *Patchwork) Debug() {
	patchwork.debug = true
}

// Patch sets the patch to be applied.
func (patchwork *Patchwork) Patch(p func(repo github.Repository, directory string)) {
	patchwork.patch = p
}

// AddRepo adds a repo to be patched.
func (patchwork *Patchwork) AddRepo(repo github.Repository) {
	patchwork.repos = append(patchwork.repos, repo)
}

// Branch sets the branch to commit to.
func (patchwork *Patchwork) Branch(branch string) {
	patchwork.branch = branch
}

// CommitMsg sets the commit message.
func (patchwork *Patchwork) CommitMsg(msg string) {
	patchwork.commitMsg = msg
}

// InitialWait sets the wait period to start at.
func (patchwork *Patchwork) InitialWait(duration time.Duration) {
	patchwork.duration = duration
}

type patchedCommit struct {
	repo github.Repository
	sha  string
}

// Apply the patch.
func (patchwork *Patchwork) Apply() {
	patchwork.logVerbose("applying patch to %d repos", len(patchwork.repos))

	patchesC := make(chan patchedCommit)
	doneBuilds := make(chan bool)

	var resultsLock sync.Mutex
	var results []circle.BuildSummary

	go func() {
		var wg sync.WaitGroup
		for patch := range patchesC {
			wg.Add(1)

			go func(patch patchedCommit) {
				defer wg.Done()

				for i := 1; i < 15; i++ {
					patchwork.logVerbose("waiting for CI of branch %s@%s", *patch.repo.FullName, patchwork.branch)
					// wait time decreases with iterations. Starts at 6 minutes.
					time.Sleep(patchwork.duration / time.Duration(i))

					patchwork.logVerbose("fetching CI status for %s@%s", *patch.repo.FullName, patchwork.branch)
					summaries, err := patchwork.circle.RecentBuildsForProjectBranch(*patch.repo.Owner.Login, *patch.repo.Name, patchwork.branch, circle.RecentBuildsOptions{
						Filter: pointers.String("completed"),
					})
					if err != nil {
						log.Fatal("couldn't get recent builds for repo", patch.repo, err)
					}

					if len(summaries) == 0 {
						patchwork.logVerbose("no completed builds for %s@%s", *patch.repo.FullName, patchwork.branch)
						continue
					}

					success := false
					for _, summary := range summaries {
						if len(summary.CommitDetails) == 0 {
							continue
						}

						if summary.CommitDetails[0].Commit == patch.sha {
							patchwork.logVerbose("successfully built commit %s for %s@%s", patch.sha, *patch.repo.FullName, patchwork.branch)
							resultsLock.Lock()
							results = append(results, summaries[0])
							resultsLock.Unlock()
							success = true
							break
						}
					}

					if success {
						break
					}
					patchwork.logVerbose("no completed builds for commit %s at %s@%s", patch.sha, patch.repo, patchwork.branch)
				}
				patchwork.logVerbose("no builds for commit %s at %s@%s", patch.sha, *patch.repo.FullName, patchwork.branch)
			}(patch)
		}
		wg.Wait()
		doneBuilds <- true
	}()

	for _, repo := range patchwork.repos {
		patchwork.logVerbose("creating temp directory for %s", *repo.FullName)
		dir, err := ioutil.TempDir("", strconv.Itoa(*repo.ID))
		if err != nil {
			log.Fatal("could not create temporary directory", err)
		}
		defer os.Remove(dir)

		patchwork.logVerbose("cloning %s", *repo.SSHURL)
		patchwork.run(dir, "git", "clone", *repo.SSHURL, dir)
		// Checking out a branch is probably unnecessary.
		patchwork.logVerbose("checking out branch %s for %s", patchwork.branch, *repo.FullName)
		patchwork.run(dir, "git", "checkout", "-b", patchwork.branch)

		if err := os.Chdir(dir); err != nil {
			log.Fatal("could not change directory", err)
		}

		patchwork.patch(repo, dir)

		patchwork.logVerbose("pushing changes to %s@%s", *repo.FullName, patchwork.branch)
		patchwork.run(dir, "git", "add", "-A")
		patchwork.run(dir, "git", "commit", "-m", "\""+patchwork.commitMsg+"\"")
		patchwork.run(dir, "git", "push", "origin", patchwork.branch)
		sha := strings.Trim(patchwork.run(dir, "git", "rev-parse", "HEAD"), "\n ")
		patchwork.logDebug("pushed commit %s to %s@%s", sha, *repo.FullName, patchwork.branch)

		patchesC <- patchedCommit{repo, sha}
	}
	close(patchesC)

	<-doneBuilds

	success := true
	for _, result := range results {
		if result.Outcome != "success" {
			success = false
			fmt.Printf("repo %s/%s failed to build\n", result.Username, result.Reponame)
		}
	}

	if !success {
		log.Fatal("There were some CI failures. Aborting.")
	}

	for _, result := range results {
		pr, _, err := patchwork.github.PullRequests.Create(result.Username, result.Reponame, &github.NewPullRequest{
			Title: &patchwork.commitMsg,
			Head:  &patchwork.branch,
			Base:  pointers.String("master"),
		})
		if err != nil {
			log.Fatal("could not create PR", err)
		}

		mergeResult, _, err := patchwork.github.PullRequests.Merge(result.Username, result.Reponame, *pr.Number, patchwork.commitMsg)
		if err != nil {
			log.Fatal("could not merge PR", err)
		}
		if !*mergeResult.Merged {
			log.Fatal("could not merge PR", err)
		}
		patchwork.logDebug("merged PR for %s/%s", result.Username, result.Reponame)
	}
}
