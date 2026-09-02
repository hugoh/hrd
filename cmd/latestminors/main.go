// Command latestminors reads newline-separated semver strings on stdin and
// prints, as a JSON array, the newest N distinct "major.minor" series (oldest
// first). It feeds the jj version matrix in the backend-compat workflow.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/hugoh/hrd/internal/versionpick"
)

const defaultMinors = 3

func main() {
	n := flag.Int("n", defaultMinors, "number of newest minor series to select")

	flag.Parse()

	var versions []string

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		versions = append(versions, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read stdin:", err)
		os.Exit(1)
	}

	series, err := versionpick.LatestMinors(versions, *n)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if series == nil {
		series = []string{}
	}

	if err := json.NewEncoder(os.Stdout).Encode(series); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
}
