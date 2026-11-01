package main

import "testing"

func TestInstallSection(t *testing.T) {
	t.Parallel()

	in := "<!-- DO NOT EDIT -->\n\n# Installing hrd\n\n## mise\n\n```sh\n# keep\nmise use -g x\n```\n"
	want := "### mise\n\n```sh\n# keep\nmise use -g x\n```\n"

	if got := installSection(in); got != want {
		t.Fatalf("installSection() = %q, want %q", got, want)
	}
}
