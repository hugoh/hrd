// Command genweb generates the static GitHub Pages homepage under site/
// from README.md (rendered to HTML) and the live Cobra command tree, so the
// site can't drift from the README or --help output. site/ is gitignored:
// this runs in CI (see .github/workflows/pages.yml) and via `go generate
// ./...` (see main.go's go:generate directive) for local preview.
package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/cmd"
	"github.com/russross/blackfriday/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	siteDir    = "site"
	readmePath = "README.md"
	repoURL    = "https://github.com/hugoh/hrd"
	siteURL    = "https://hrd.larve.net/"
	siteName   = "hrd"
	dirPerm    = 0o750
	filePerm   = 0o600
)

// headingLineRE matches Markdown heading lines, used to find the tagline:
// the first non-blank, non-heading, non-image/link-only line in README.md.
var headingLineRE = regexp.MustCompile(`^#{1,6}\s`)

func run() error {
	git.Register()
	jj.Register()

	if err := os.MkdirAll(siteDir, dirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", siteDir, err)
	}

	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", readmePath, err)
	}

	data := pageData{
		SiteName:      siteName,
		RepoURL:       repoURL,
		SiteURL:       siteURL,
		Description:   tagline(readme),
		ReadmeHTML:    renderMarkdown(readme),
		Commands:      collectCommands(cmd.NewApp()),
		PicoVersion:   picoVersion,
		PicoIntegrity: picoIntegrity,
	}

	if err := writeIndex(data); err != nil {
		return err
	}

	if err := writeFile(filepath.Join(siteDir, "style.css"), styleCSS); err != nil {
		return err
	}

	if err := writeFile(filepath.Join(siteDir, "robots.txt"), robotsTxt(data.SiteURL)); err != nil {
		return err
	}

	return writeFile(filepath.Join(siteDir, "sitemap.xml"), sitemapXML(data.SiteURL))
}

// tagline extracts the first non-blank, non-heading line from README.md to
// use as the page's meta description: README.md.tpl always opens with an
// "# hrd" heading followed by a one-line tagline paragraph.
func tagline(readme []byte) string {
	for line := range strings.SplitSeq(string(readme), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || headingLineRE.MatchString(line) || strings.HasPrefix(line, "[") {
			continue
		}

		return line
	}

	return ""
}

func renderMarkdown(src []byte) template.HTML {
	exts := blackfriday.CommonExtensions | blackfriday.AutoHeadingIDs
	rendered := blackfriday.Run(src, blackfriday.WithExtensions(exts))

	return template.HTML(rendered) //nolint:gosec // src is our own committed README.md
}

// commandDoc is one Cobra command rendered for the "Commands" reference
// section of the homepage.
type commandDoc struct {
	Path  string // full space-separated command path, e.g. "hrd repo add"
	Short string
	Long  string
	Flags []flagDoc
}

type flagDoc struct {
	Name      string
	Shorthand string
	Default   string
	Usage     string
}

// collectCommands walks the live Cobra tree depth-first, in the same order
// commands were registered (see cmd.buildCommands), skipping the commands
// Cobra adds automatically (help, completion).
func collectCommands(root *cobra.Command) []commandDoc {
	var docs []commandDoc

	var walk func(c *cobra.Command)

	walk = func(c *cobra.Command) {
		if c.Name() != "help" && c.Name() != "completion" {
			docs = append(docs, commandDoc{
				Path:  c.CommandPath(),
				Short: c.Short,
				Long:  c.Long,
				Flags: collectFlags(c),
			})
		}

		for _, sub := range c.Commands() {
			walk(sub)
		}
	}

	walk(root)

	return docs
}

func collectFlags(c *cobra.Command) []flagDoc {
	var flags []flagDoc

	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}

		flags = append(flags, flagDoc{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Default:   anonymizeHomeDir(f.DefValue),
			Usage:     f.Usage,
		})
	})

	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })

	return flags
}

// anonymizeHomeDir rewrites a flag default like the -c/--config path, which
// Cobra renders as the actual absolute path on the machine that ran this
// generator, to use "~" instead. Without this, every regeneration would embed
// whoever ran it's home directory, making the committed site differ machine
// to machine and breaking the gen-check CI diff.
func anonymizeHomeDir(s string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return s
	}

	return strings.ReplaceAll(s, home, "~")
}

func writeFile(path string, content string) error {
	if err := os.WriteFile(path, []byte(content), filePerm); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")

		os.Exit(1)
	}
}
