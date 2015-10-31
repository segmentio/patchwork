package patchwork

import (
	"bytes"
	"log"
	"os/exec"
)

// run will run the command `name` in the given `dir` directory with the given
// arguments. It also logs the output of the command in case of a failure.
func (patchwork *Patchwork) run(dir, name string, args ...string) string {
	patchwork.logVerbose("running %s %s", name, args)
	command := exec.Command(name, args...)
	var buf bytes.Buffer
	command.Stdout = &buf
	command.Stderr = &buf
	command.Dir = dir
	if err := command.Run(); err != nil {
		patchwork.logError("could not run %v %v", name, args)
		log.Fatal(buf.String())
	}
	return buf.String()
}
