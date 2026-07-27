package okf

import (
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

// Validator is the behavioral opt-in for types that contribute custom
// conformance rules (see Bundle.Conformance). It is deliberately not part
// of the registry contract: a registered type only needs to implement it
// when it has extra rules to enforce.
type Validator interface {
	Validate() []Violation
}

var (
	registryMu sync.RWMutex
	registry   = map[string]func(*Concept) (any, error){}
)

// Register adds a decoder for a concept `type` string to the package-level
// registry, modifiable externally at init() (modeled on image.RegisterFormat
// / database/sql.Register / Kubernetes' Scheme). go-okf ships exactly one
// built-in registration ("Attested Computation"); every other type string
// is free-form and consumers register their own decoders with zero changes
// to this package.
func Register(kind string, decode func(*Concept) (any, error)) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[kind] = decode
}

// Typed dispatches to the decoder registered for c.Type, if any. It
// returns (nil, false) when no decoder is registered for the type, or when
// the registered decoder returns an error — callers that need the error
// should call the registered decode func (or As[T]) directly.
func (c *Concept) Typed() (any, bool) {
	registryMu.RLock()
	decode, ok := registry[c.Type]
	registryMu.RUnlock()
	if !ok {
		return nil, false
	}
	v, err := decode(c)
	if err != nil {
		return nil, false
	}
	return v, true
}

// As decodes the concept's merged full frontmatter (core fields plus
// Extra — every key) into T via yaml struct tags, so a caller-known type
// sees both the OKF core fields (title, description, ...) and its own
// custom keys. This is the compile-time path: it needs no registry entry.
// For decoding that As can't express, supply a custom func to Register
// instead.
func As[T any](c *Concept) (*T, error) {
	b, err := yaml.Marshal(c.frontmatterMap())
	if err != nil {
		return nil, fmt.Errorf("okf: marshaling frontmatter for As: %w", err)
	}
	var out T
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("okf: decoding frontmatter into %T: %w", out, err)
	}
	return &out, nil
}
