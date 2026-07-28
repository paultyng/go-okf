# go-okf

[![Go Reference](https://pkg.go.dev/badge/github.com/paultyng/go-okf.svg)](https://pkg.go.dev/github.com/paultyng/go-okf)

A standalone Go implementation of the [Open Knowledge Format (OKF)](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).

OKF v0.2 models a knowledge bundle as a directory of markdown files with YAML frontmatter, where each file is one concept and reserved `index.md` / `log.md` files provide directory listings and change history. go-okf is a pure-Go, dependency-light library for working with these bundles: parse and round-trip single concepts, load a whole bundle over an [`io/fs.FS`](https://pkg.go.dev/io/fs#FS), extract and deduplicate links/citations/resources, synthesize index content, and check conformance. Typed concepts are layered on via an extensible registry (`Register` / `As` / `Concept.Typed`) rather than a closed interface hierarchy.

## Install

```sh
go get github.com/paultyng/go-okf
```

## Usage

Load a bundle from a directory and list its concepts:

```go
package main

import (
	"fmt"
	"os"

	okf "github.com/paultyng/go-okf"
)

func main() {
	b, err := okf.Load(os.DirFS("./my-bundle"))
	if err != nil {
		panic(err)
	}

	for _, c := range b.Concepts() {
		fmt.Printf("%s: %s\n", c.Type, c.Title)
	}
}
```

Or parse a single concept and write it back:

```go
c, err := okf.Parse(data)
if err != nil {
	panic(err)
}
fmt.Println(c.Title)

out := c.Bytes() // frontmatter + verbatim body, ready to persist
```

## Status

v0.2, beta (`v0.1.0-beta.2` tagged). [MIT licensed](LICENSE).
