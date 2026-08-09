package kubeexec

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

var (
	verbose bool
	logOut  io.Writer = io.Discard
	mu      sync.Mutex
)

// SetVerbose enables or disables debug logging of executed commands.
// When enabled, commands and their outputs are written to the provided
// writer (typically os.Stderr). When disabled (the default), all logging
// is discarded.
func SetVerbose(enabled bool, w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	verbose = enabled
	if w != nil {
		logOut = w
	}
}

func logCmd(name string, args ...string) {
	mu.Lock()
	defer mu.Unlock()
	if !verbose {
		return
	}
	fmt.Fprintf(logOut, "[cmd] %s %s\n", name, strings.Join(args, " "))
}

func logOutput(name string, output string) {
	mu.Lock()
	defer mu.Unlock()
	if !verbose {
		return
	}
	if output != "" {
		fmt.Fprintf(logOut, "[out] %s: %s\n", name, output)
	}
}

func logErr(name string, err error) {
	mu.Lock()
	defer mu.Unlock()
	if !verbose {
		return
	}
	fmt.Fprintf(logOut, "[err] %s: %v\n", name, err)
}
