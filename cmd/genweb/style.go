package main

// picoVersion pins the Pico CSS release loaded from jsdelivr in page.go's
// template. Bump picoIntegrity (sha384 SRI hash) alongside it when updating.
const (
	picoVersion   = "2.1.1"
	picoIntegrity = "sha384-NZhm4G1I7BpEGdjDKnzEfy3d78xvy7ECKUwwnKTYi036z42IyF056PbHfpQLIYgL"
)

const styleCSS = `
.topnav {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 1.5rem;
  padding: 0.9rem var(--pico-spacing);
  margin-bottom: 0;
  background: color-mix(in srgb, var(--pico-background-color) 85%, transparent);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--pico-muted-border-color);
}

.topnav a {
  text-decoration: none;
}

.topnav .brand {
  margin-right: auto;
  color: var(--pico-h1-color, var(--pico-color));
  font-weight: 700;
}

h1, h2, h3, #commands {
  scroll-margin-top: 4.5rem;
}

#commands {
  margin-top: 3rem;
  border-top: 1px solid var(--pico-muted-border-color);
  padding-top: 1.5rem;
}

.command {
  border-bottom: 1px solid var(--pico-muted-border-color);
  padding: 1rem 0;
}

.command h3 {
  margin-bottom: 0.4rem;
}

.flags {
  margin: 0.6rem 0 0;
}

.flags dd {
  color: var(--pico-muted-color);
}
`
