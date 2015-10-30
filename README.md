# patchwork

Patchwork is a library to that lets you apply a single change across mutliple Github repositories.

```go
repos := []string{"foo", "bar"}
patchwork.Apply(repos, func(repo *github.Repo, directory string) string {
  // Apply your patch here.
  return "updating godeps"
})
```

Patchwork will clone your repos, checkout a new branch, apply your patch with the given commits,
open a Pull Request, wait on Circle CI to build it, and finally prompt you to apply the patch.
