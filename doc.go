// Package okf implements the Open Knowledge Format (OKF) v0.2: a directory
// of markdown files with YAML frontmatter representing a knowledge bundle.
// See the [OKF spec] for the format definition.
//
// The package is pure OKF: it has zero dependencies on any specific
// consumer's domain types. Extensibility is provided via a type registry
// ([Register], [As], [Concept.Typed]) rather than a closed interface hierarchy;
// [AttestedComputation] is the one built-in typed concept.
//
// Parse a single concept with [Parse] and write it back with [Concept.Bytes];
// load a whole bundle from an [io/fs.FS] with [Load]. The reserved
// index.md and log.md files are handled by [Index] and [Log].
// [Bundle.ExternalLinks] derives a bundle's outbound resources, and
// [Bundle.Conformance] checks OKF conformance.
//
// Core has no concrete filesystem coupling beyond stdlib [io/fs.FS] for
// reads; writes are caller-owned (the Bytes methods on [Concept], [Index],
// and [Log] return bytes, callers persist them). Core never calls
// [time.Now]: dates are passed in by the caller for determinism.
//
// [OKF spec]: https://github.com/GoogleCloudPlatform/knowledge-catalog/tree/main/okf
package okf
