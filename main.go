package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

//go:embed manifest.json
var manifestData []byte

//go:embed example.md
var exampleData []byte

// Frontmatter holds the YAML frontmatter fields.
type Frontmatter struct {
	Title string `yaml:"title"`
}

func main() {
	showManifest := flag.Bool("manifest", false, "output embedded manifest.json")
	showExample := flag.Bool("example", false, "output embedded example.md")
	showVersion := flag.Bool("version", false, "output version from manifest")
	flag.Parse()

	if *showVersion {
		var m map[string]interface{}
		if err := json.Unmarshal(manifestData, &m); err == nil {
			if v, ok := m["version"]; ok {
				fmt.Println(v)
			}
		}
		return
	}

	if *showManifest {
		os.Stdout.Write(manifestData)
		return
	}
	if *showExample {
		os.Stdout.Write(exampleData)
		return
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}

	fm, body := splitFrontmatter(input)

	var meta Frontmatter
	if len(fm) > 0 {
		if err := yaml.Unmarshal(fm, &meta); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing frontmatter: %v\n", err)
			os.Exit(1)
		}
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	writePageSetup(w, &meta)
	renderBody(w, body)
}

// splitFrontmatter separates YAML frontmatter (between --- delimiters) from the body.
func splitFrontmatter(data []byte) (frontmatter, body []byte) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, data
	}

	// Find the closing ---
	rest := s[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, data
	}

	fm := rest[:idx]
	bodyStart := idx + 4 // skip "\n---"
	if bodyStart < len(rest) && rest[bodyStart] == '\n' {
		bodyStart++
	} else if bodyStart < len(rest) && rest[bodyStart] == '\r' {
		bodyStart++
		if bodyStart < len(rest) && rest[bodyStart] == '\n' {
			bodyStart++
		}
	}

	return []byte(fm), []byte(rest[bodyStart:])
}

// writePageSetup outputs the Typst page setup and metadata.
func writePageSetup(w *bufio.Writer, meta *Frontmatter) {
	fmt.Fprintln(w, `#set page(paper: "a4")`)
	fmt.Fprintln(w, `#set text(font: "SimSun", size: 12pt, lang: "zh")`)
	fmt.Fprintln(w, `#set par(leading: 1.5em, first-line-indent: 2em)`)
	fmt.Fprintln(w)

	if meta.Title != "" {
		fmt.Fprintf(w, "#let title = %q\n", meta.Title)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "#align(center, text(size: 22pt, weight: \"bold\")[%s])\n", meta.Title)
		fmt.Fprintln(w, `#v(1em)`)
		fmt.Fprintln(w)
	}
}

// renderBody parses the Markdown body using goldmark and outputs Typst.
func renderBody(w *bufio.Writer, source []byte) {
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch node := n.(type) {
		case *ast.Heading:
			if entering {
				fmt.Fprintf(w, "#heading(level: %d)[", node.Level)
			} else {
				fmt.Fprintln(w, "]")
				fmt.Fprintln(w)
			}

		case *ast.Paragraph:
			if !entering {
				fmt.Fprintln(w)
				fmt.Fprintln(w)
			}

		case *ast.Text:
			if entering {
				w.Write(node.Segment.Value(source))
				if node.SoftLineBreak() {
					fmt.Fprintln(w)
				}
			}

		case *ast.List:
			if !entering {
				fmt.Fprintln(w)
			}

		case *ast.ListItem:
			if entering {
				fmt.Fprint(w, "- ")
			} else {
				fmt.Fprintln(w)
			}

		case *ast.Emphasis:
			if node.Level == 2 {
				if entering {
					fmt.Fprint(w, "#strong[")
				} else {
					fmt.Fprint(w, "]")
				}
			} else {
				if entering {
					fmt.Fprint(w, "#emph[")
				} else {
					fmt.Fprint(w, "]")
				}
			}

		case *ast.ThematicBreak:
			if entering {
				fmt.Fprintln(w, "#line(length: 100%)")
				fmt.Fprintln(w)
			}

		case *ast.CodeSpan:
			if entering {
				fmt.Fprintf(w, "#raw(\"%s\")", string(node.Text(source)))
			}

		case *ast.FencedCodeBlock:
			if entering {
				var buf bytes.Buffer
				lines := node.Lines()
				for i := 0; i < lines.Len(); i++ {
					line := lines.At(i)
					buf.Write(line.Value(source))
				}
				content := strings.TrimRight(buf.String(), "\n")
				fmt.Fprintf(w, "```\n%s\n```\n\n", content)
			}
		}

		return ast.WalkContinue, nil
	})
}
