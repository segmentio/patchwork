# patchwork

Patchwork is a library to that lets you apply a single change across mutliple Github repositories.

# Usage
```go
p := patchwork.New(os.Getenv("GITHUB_TOKEN"), os.Getenv("CIRCLE_TOKEN"))

opts := &patchwork.ApplyOptions{}
opts.Message = "Some Commit Message for this change."
opts.Branch = "some-branch-for-this-update" // should be a branch that has not been created recently.
opts.Repos = []patchwork.Repository{
  // An array of repos to update.
  {"segment-integrations", "analytics.js-integration-mixpanel"},
}

p.Apply(*opts, func(repo *github.Repository, dir string) {
  // apply some changes here.
})
```

Patchwork will clone your repos, apply your patch, push to a new branch and wait for their CI results.
If any tests fail, it will print a message indicating that the commit failed.
If all tests succeed, it will open a Pull Request, and merge it for you.
