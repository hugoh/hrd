package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

// pageData feeds the index.html template.
type pageData struct {
	SiteName      string
	RepoURL       string
	SiteURL       string
	Description   string
	ReadmeHTML    template.HTML
	Commands      []commandDoc
	PicoVersion   string
	PicoIntegrity string
}

//nolint:gochecknoglobals // parsed once at package init, read-only thereafter
var indexTemplate = template.Must(template.New("index.html").Parse(indexHTMLTpl))

func writeIndex(data pageData) error {
	out, err := os.Create(filepath.Join(siteDir, "index.html"))
	if err != nil {
		return fmt.Errorf("creating index.html: %w", err)
	}

	if err := indexTemplate.Execute(out, data); err != nil {
		_ = out.Close()

		return fmt.Errorf("executing index.html template: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("closing index.html: %w", err)
	}

	return nil
}

func robotsTxt(siteURL string) string {
	return fmt.Sprintf("User-agent: *\nAllow: /\nSitemap: %ssitemap.xml\n", siteURL)
}

func sitemapXML(siteURL string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>%s</loc>
  </url>
</urlset>
`, siteURL)
}

const indexHTMLTpl = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.SiteName}} — {{.Description}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.SiteURL}}">
<meta property="og:type" content="website">
<meta property="og:title" content="{{.SiteName}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.SiteURL}}">
<link rel="stylesheet"
  href="https://cdn.jsdelivr.net/npm/@picocss/pico@{{.PicoVersion}}/css/pico.classless.min.css"
  integrity="{{.PicoIntegrity}}" crossorigin="anonymous">
<link rel="stylesheet" href="style.css">
</head>
<body>
<nav class="topnav">
<a class="brand" href="#">{{.SiteName}}</a>
<a href="#features">Features</a>
<a href="#install">Install</a>
<a href="#commands">Commands</a>
<a href="{{.RepoURL}}">GitHub</a>
</nav>
<main>
{{.ReadmeHTML}}

<section id="commands">
<h2>Commands</h2>
{{range .Commands}}
<article class="command">
<h3><code>{{.Path}}</code></h3>
{{if .Short}}<p>{{.Short}}</p>{{end}}
{{if .Long}}<p>{{.Long}}</p>{{end}}
{{if .Flags}}
<dl class="flags">
{{range .Flags}}
<dt><code>{{if .Shorthand}}-{{.Shorthand}}, {{end}}--{{.Name}}</code></dt>
<dd>{{.Usage}}{{if .Default}} (default: {{.Default}}){{end}}</dd>
{{end}}
</dl>
{{end}}
</article>
{{end}}
</section>
</main>
<footer>
<p>Source, issues, and releases on <a href="{{.RepoURL}}">GitHub</a>.</p>
</footer>
</body>
</html>
`
