package main

import "testing"

func TestEnsureNonBlankTypstRejectsBlankOutput(t *testing.T) {
	if err := ensureNonBlankTypst(" \n\t"); err == nil {
		t.Fatal("expected blank Typst output to fail")
	}
}

func TestEnsureNonBlankTypstAcceptsOutput(t *testing.T) {
	if err := ensureNonBlankTypst("#set page()\n"); err != nil {
		t.Fatalf("expected non-blank Typst output to pass, got %v", err)
	}
}
