package patchwork

import (
	"fmt"
	"log"
)

// Conditionally log the given message.
func (patchwork *Patchwork) logDebug(format string, v ...interface{}) {
	if patchwork.debug {
		log.Printf("\033[34m%s\033[0m\n", fmt.Sprintf(format, v...))
	}
}

func (patchwork *Patchwork) logVerbose(format string, v ...interface{}) {
	if patchwork.debug {
		log.Printf(format, v...)
	}
}

func (patchwork *Patchwork) logError(format string, v ...interface{}) {
	if patchwork.debug {
		log.Printf("\033[31m%s\033[0m\n", fmt.Sprintf(format, v...))
	}
}
