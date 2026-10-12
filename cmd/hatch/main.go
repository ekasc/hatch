// Command hatch boots headless app instances.
//
//	hatch run --bundle <bundle.js> [--engine node] [--api http://host:port] [-- arg...]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ekassinghchhabra/hatch/internal/instance"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, "usage: hatch run --bundle <bundle.js> [--engine <bin>] [--api <url>] [-- arg...]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	bundle := fs.String("bundle", "", "path to the JS bundle to execute")
	engine := fs.String("engine", "node", "JS engine binary (Hermes CLI when available; node until then)")
	api := fs.String("api", "", "backend base URL, exposed as HATCH_API_URL")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	res, err := instance.Run(instance.Config{
		Engine: *engine,
		Bundle: *bundle,
		APIURL: *api,
		Args:   fs.Args(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hatch:", err)
		os.Exit(1)
	}
	os.Stdout.WriteString(res.Stdout)
	os.Stderr.WriteString(res.Stderr)
	os.Exit(res.ExitCode)
}
