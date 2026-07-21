package main

const styleCSS = `
:root {
  color-scheme: light dark;
  --fg: #1a1a1a;
  --bg: #ffffff;
  --muted: #5a5a5a;
  --accent: #2453ff;
  --border: #e2e2e2;
  --code-bg: #f4f4f4;
}

@media (prefers-color-scheme: dark) {
  :root {
    --fg: #e8e8e8;
    --bg: #14161a;
    --muted: #9a9a9a;
    --accent: #7ba1ff;
    --border: #2c2f36;
    --code-bg: #1d2026;
  }
}

* { box-sizing: border-box; }

body {
  margin: 0;
  padding: 0 0 3rem;
  background: var(--bg);
  color: var(--fg);
  font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
}

.topnav {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 1.5rem;
  padding: 0.9rem 1.25rem;
  background: color-mix(in srgb, var(--bg) 85%, transparent);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--border);
}

.topnav a {
  color: var(--fg);
  text-decoration: none;
  font-size: 0.95rem;
}

.topnav a:hover {
  color: var(--accent);
}

.topnav .brand {
  font-weight: 700;
  margin-right: auto;
}

main {
  max-width: 46rem;
  margin: 0 auto;
  padding: 0 1.25rem;
  scroll-padding-top: 4rem;
}

h1, h2, h3 {
  line-height: 1.25;
  scroll-margin-top: 4.5rem;
}

h1 {
  margin-top: 2.5rem;
  font-size: 2.25rem;
}

a {
  color: var(--accent);
}

code, pre {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  background: var(--code-bg);
  border-radius: 4px;
}

code {
  padding: 0.1em 0.35em;
}

pre {
  padding: 0.9rem 1rem;
  overflow-x: auto;
}

pre code {
  padding: 0;
  background: none;
}

img {
  max-width: 100%;
}

#commands {
  margin-top: 3rem;
  border-top: 1px solid var(--border);
  padding-top: 1.5rem;
}

.command {
  padding: 1rem 0;
  border-bottom: 1px solid var(--border);
}

.command h3 {
  margin-bottom: 0.4rem;
  font-size: 1.05rem;
}

.flags {
  margin: 0.6rem 0 0;
}

.flags dt {
  font-size: 0.9rem;
}

.flags dd {
  margin: 0 0 0.5rem 0;
  color: var(--muted);
  font-size: 0.9rem;
}

footer {
  max-width: 46rem;
  margin: 2.5rem auto 0;
  padding: 0 1.25rem;
  color: var(--muted);
  font-size: 0.9rem;
}
`
