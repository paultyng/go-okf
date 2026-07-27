# OKF prior art

Sources fetched 2026-07-22/27 via raw.githubusercontent.com + api.github.com.
Both repos reachable.

## W4G1/okf (Rust)

Targets **OKF v0.2** (our lib targets v0.1 — v0.2 adds `sources`/trust/lifecycle/
Attested Computation frontmatter; treat those sections as forward-looking, not
in-scope for v0.1). Files read: `src/document.rs`, `src/bundle.rs`, `src/links.rs`,
`src/concept_id.rs`, `src/validate.rs` (partial), `src/frontmatter.rs` (signatures
only), `src/error.rs`, README.

### Data model / types
- `Document { frontmatter: Frontmatter, body: String }` — parse/serialize pair.
  `Frontmatter` wraps an order-preserving YAML `Mapping` + typed getters
  (`type_()`, `title()`, `tags()`, …) layered on top — never a fixed struct.
- `Concept` = a loaded, bundle-resident `Document` plus derived accessors
  (`type_()`, `trust_tier()`, `sources()`) — i.e. bundle-loaded concepts are a
  distinct (thin) type from a standalone parsed `Document`.
- `ConceptId { segments: Vec<String> }`: parsed from `/`-path, `to_path`/`from_path`
  convert to/from filesystem paths (`.md` suffix stripped/added). `parse()` validates
  segments (rejects `/`, `\`, control chars, `.`/`..`); `from_path()` does **not**
  validate — "a file already on disk is a concept whatever it is called."
  ASCII-portability (`[A-Za-z0-9_][A-Za-z0-9_.-]*`) is checked separately as
  `is_portable_segment` (warning-only, not a parse error).
- `Bundle`: `load()` walks a dir, builds concepts + reserved file lists
  (`RESERVED_FILENAMES = ["index.md", "log.md"]`) + two graphs: cross-link graph
  (from markdown links) with backlinks, and a derivation graph (from
  `sources[].resource` pointing at another concept). `parse_errors()` collects
  per-file failures rather than aborting load.
- `Link { text, target, kind: LinkKind }` where `LinkKind` = Absolute (`/`-prefixed,
  bundle-root-relative) / Relative (`./…`) / External (URI scheme or `//`) / Anchor
  (`#…`) / Other. `Link::resolve_all()` returns **candidates in priority order**
  (literal target, then percent-decoded) rather than a single answer — caller picks
  first that exists in the bundle.
- `field_path_candidates(raw, from)` — separate resolution path for **frontmatter**
  path-valued fields (`resource`, etc.), which the spec resolves against two
  different bases (concept-relative vs. bundle-root-relative); returns both
  candidates, "callers take the first that exists."
- Validation: `Document::validate()` checks only non-empty `type` (matches §11:
  the ONLY hard conformance requirement). `missing_recommended()` is a separate,
  non-failing check for `title`/`description`/etc., returned as advisory list, not
  errors. `validate_bundle()` returns a `Report` of severity-tagged `Diagnostic`s
  (Error/Warning/Info) — errors are strictly the `type`-missing / reserved-file-
  shape / unterminated-frontmatter class; almost everything else is warning/info.
- Errors: `DocumentError` (`UnterminatedFrontmatter`, `FrontmatterNotMapping`,
  `InvalidYaml`, `MissingKeys(Vec<String>)`) and `BundleError` (`Io`,
  `NotADirectory`, `Document{path,error}`, `InvalidConceptId{path,reason}`) —
  both plain enums with `Display`+`Error` impls, no anyhow-style dynamic errors.
- `Document::parse`: a file with no leading `---` line is treated as **all-body,
  empty frontmatter** (not an error). An opened-but-unterminated `---` block IS
  an error.
- Link/citation extraction skips fenced code blocks and inline code spans before
  regex/char-scanning for `[text](dest)` — content in code is not a relationship.
- `is_external()` recognizes **any** RFC-3986 scheme (`bigquery:...`), not just
  `http(s)` — non-URL warehouse resource identifiers are common in `resource`.

### Adopt
- Order-preserving frontmatter mapping + typed getters over it, not a fixed
  struct — directly validates our planned "preserve unknown keys" design.
- Reserved filenames as a named constant/set (`index.md`, `log.md`), checked
  once in the loader, not re-derived per call site.
- Permissive `Bundle.Load`: collect per-file parse errors, keep loading; keep
  broken link edges (don't drop them) so a bundle's incompleteness is visible,
  not silently dropped.
- `ConceptId.parse` (strict) vs. `ConceptId.FromPath` (permissive, no validation)
  as two different entry points — matches "reading is more permissive than
  authoring" and avoids losing legitimately-odd files.
- Two-tier link classification: `LinkKind` (Absolute/Relative/External/Anchor/Other)
  plus a separate `resolve_all` returning ordered candidates (literal, then
  percent-decoded) rather than a single guess.
- Distinguish concept-relative vs. bundle-root-relative resolution for
  **frontmatter path fields** vs. body markdown links — these use different
  bases per spec and conflating them silently mis-resolves `resource`.
- `is_external` keyed on generic URI-scheme detection, not a `http(s)` allowlist.
- Plain typed error enums (no dynamic/opaque errors) with clear `Display` text —
  matches idiomatic Go `errors.Is`/sentinel-friendly design.
- Skip code fences/inline-code spans before link extraction.
- Validation split into hard-fail (`type` only) vs. advisory (`missing_recommended`)
  as two distinct methods, not one call with a severity parameter — keeps the
  "what makes a bundle non-conformant" surface minimal and explicit.

### Reject / N/A
- Everything under `provenance`/`trust`/`computation`/`actor`/`footnotes` modules —
  pure v0.2 additions (trust tiers, staleness, Attested Computation contracts,
  actor convention). Out of scope for a v0.1 library; would be premature surface.
- `Bundle::stale_on(today)` / `TrustTier` comparisons — depends on v0.2 fields.
- CLI subcommand surface (`validate`/`lint`/`trust`/`graph`/`diff`/`fmt`) — that's
  a full CLI tool, not a library concern; our contract is library-only.
- Reordering frontmatter into a "preferred key order" (`reorder_preferred`) —
  cute for matching a reference implementation's byte-output, unneeded complexity
  for us since we're not targeting byte-identical round-trip with a Python
  reference.
- Custom hand-rolled YAML subset parser (`src/yaml/`) — we already committed to
  yaml.v3, no need to reinvent a YAML subset in Go.

## xSAVIKx/okf-skills (Go)

Files read: `okf-go/okf.go`, `okf-go/sections.go`, `okf-go/lint.go` (partial),
`okf-go/relationships.go`, `okf-go/log.go`, `okf-go/semantic.go`, root README.
Note: this is an agentic-skills toolkit (SQL/git/CSV connectors + MCP server)
producing OKF bundles from external data sources — almost all of it (connectors,
enrichment guidance, MCP tool wiring) is out of scope. Only `okf-go/` (the shared
library) is relevant to reading/writing bundles.

### Data model / types
- `Frontmatter` is a **fixed Go struct** with `yaml:"…,omitempty"` tags
  (`Type`, `Title`, `Description`, `Resource`, `Tags`, `Timestamp`, plus
  connector-specific `ContentHash`, `EnrichedAgainst`, `OKFVersion`). Unmarshaling
  via `yaml.Unmarshal(parts[1], &fm)` — **unknown keys are silently dropped**,
  not preserved. This directly contradicts our "preserve unknown frontmatter
  keys" requirement.
- `ConceptDoc { Frontmatter Frontmatter, Body string }`; `ReadConceptDoc`/
  `WriteConceptDoc` operate directly on a file path (not `io/fs.FS`), splitting
  on a literal byte-string delimiter via
  `bytes.SplitN(content, []byte("---\n"), 3)`, with a second full pass using
  `"---\r\n"` if the first yields fewer than 3 parts. This is a byte-substring
  match, not a line-based scan: it assumes the file is pure-LF or pure-CRLF
  throughout and requires frontmatter to start at byte 0.
- `IgnoreMatcher`/`ReadFolderMetadata` — connector-specific (`.okfignore`,
  `.okf-metadata.yaml`), no spec basis; skip for our lib.
- `UpsertSection(body, heading, content)` / `GetSection` / `GetSectionAny` —
  markdown **level-2 section replace-or-append** by heading text, line-based
  scan (not AST-based). `GetSectionAny` matches heading text at any ATX level
  (`#`/`##`) since connectors emit `# Columns` (level-1) but the general
  section API expects `## heading` (level-2) — a two-tier "the effective
  heading level is inconsistent across producers" workaround.
- `Relationship{Label, Target, Text}` + `RenderRelationshipsSection` — renders
  a sorted (`Target, Label, Text`) bullet list of markdown links for
  determinism/reproducibility (re-running a connector is a byte-identical no-op).
- `AppendLogEntry(bundleDir, date, kind, message)` — the `log.md` writer: newest-
  date-first `"## YYYY-MM-DD"` headings, `"* **<Kind>**: <message>"` bullets,
  same-day entries prepended under the existing heading. Caller supplies `date`
  (not `time.Now()` internally) to keep it deterministic/testable.
- `LintReport`/`Finding{Rule, Path, Detail}` — conformance findings keyed by a
  stable machine-readable `Rule` string constant (`RuleMissingType`,
  `RuleRootIndexExtraKeys`, …), one `Finding` struct shape reused for all rules.
- `DetectSemanticType`/`ClassifyColumn` — connector-specific column-type
  inference (email/uuid/enum/fk-ish); irrelevant to a generic OKF library.

### Adopt
- `AppendLogEntry`'s exact `log.md` shape and merge logic (group by date,
  newest day first, newest entry within a day first, caller-supplied date for
  determinism) — good concrete algorithm to mirror for our `Log` handler.
- `Finding{Rule, Path, Detail}` with a package of named `Rule*` string constants
  for `Conformance() []Violation` — stable, greppable rule ids beat unstructured
  messages; adopt the shape (rename `Finding`→`Violation` per our contract).
- Sorting all report output for byte-stable/deterministic diffs (`sort.Slice` on
  `(Target, Label, Text)` etc.) — apply the same discipline to `ExternalLinks()`/
  `Conformance()` output ordering.
- The `# Columns` vs `## heading` ambiguity they hit (`GetSectionAny`) is a
  useful cautionary data point: markdown "section" boundaries are producer-
  convention, not spec-enforced — if we add any section-extraction helper,
  don't hardcode a single heading level.

### Reject / N/A
- Fixed-struct `Frontmatter` with struct tags — **directly reject**: drops
  unknown keys on unmarshal, violates round-trip/preservation requirement our
  contract already commits to. This is the single clearest negative precedent
  found in either repo for our design.
- Byte-prefix `SplitN` frontmatter parsing instead of a line-based scan — reject;
  less robust than a line-oriented split (what W4G1 and our planned hand-rolled
  splitter both do) and doesn't cleanly reject an unterminated frontmatter block
  as a distinct error case.
- File-path-based `ReadConceptDoc(filePath string)` / `WriteConceptDoc` — reject
  as the primary API; we've already committed to `io/fs.FS`-based `Bundle.Load`,
  which additionally isn't tied to a real filesystem (works for embed.FS, zip,
  testing fixtures).
- No concept-id/path abstraction at all (paths are raw strings throughout,
  `resolveBundleLink` is a simple string-join) — no `ConceptId` type comparable
  to W4G1's; not worth adopting as-is, but confirms an id/path type is worth
  having (see W4G1 adopt list) since this repo's string-based approach visibly
  needed ad hoc special-casing (CRLF fallback, ToSlash calls) sprinkled around.
- Everything connector/MCP/enrichment/semantic-detection related — out of scope,
  not a bundle-reading/writing concern.

## Net recommendations for go-okf

- **Confirmed**: preserve-unknown-keys frontmatter design is correct and is the
  #1 differentiator from xSAVIKx's approach — its fixed-struct + `omitempty`
  silently drops any producer-defined key on round-trip. Keep an
  order-preserving map (e.g. `yaml.Node` or `yaml.MapSlice`-equivalent) under
  typed accessor methods (`Type()`, `Title()`, …), not a plain struct.
  `yaml.v3`'s `yaml.Node` preserves both order and comments/style if we need it.
- **Confirmed**: `io/fs.FS`-based `Bundle.Load` is the right call — neither
  reference library commits to this (W4G1 uses `Path`, xSAVIKx uses raw file
  paths); ours is a genuine improvement for testability (embed.FS, in-memory
  fixtures) and matches Go idiom better than either prior art.
- **Adopt for `Conformance() []Violation`**: keep it to the spec's single hard
  requirement (non-empty `type`) as ERROR-level; put everything else (missing
  title/description, unknown reserved-file shape, broken links, non-portable
  segment names) at WARNING/INFO with a stable machine-readable `Rule` string
  per violation (borrow xSAVIKx's `Finding{Rule,Path,Detail}` shape, rename to
  our `Violation`). Don't let `Conformance()` reject on anything beyond `type`.
- **Adopt for concept ids / paths**: mirror W4G1's split — a permissive
  path→id derivation (accept whatever's on disk) vs. a stricter parse/construct
  path for id validation, plus a separate non-fatal "portable ASCII segment"
  guidance check. Confirms our planned model needs an explicit `ConceptID`-like
  type rather than raw strings (xSAVIKx's raw-string approach visibly needed
  scattered CRLF/slash workarounds).
- **Adopt for `ExternalLinks()`**: classify by generic URI-scheme detection
  (`scheme:` per RFC 3986), not an `http(s)` allowlist — `resource` fields
  legitimately use non-HTTP schemes (e.g. `bigquery:project.dataset.table`).
  Skip fenced code blocks/inline code spans before scanning body links (or rely
  on goldmark's AST, which already does this for us — a place where goldmark
  gives us a real edge over both hand-rolled scanners).
- **Adopt for link/resource resolution**: return resolution as an **ordered
  candidate list** (literal path, then percent-decoded; concept-relative, then
  bundle-root-relative for frontmatter path fields) rather than a single
  resolved value — matches W4G1's `resolve_all`/`field_path_candidates` and
  reflects genuine spec ambiguity, not implementation laziness.
- **`CanonicalURL` (comparison-only)**: neither prior-art repo implements URL
  canonicalization for `resource`/link comparison as a first-class concept —
  this is genuinely new ground for us; no existing pattern to lift. Keep our
  own design (comparison-only, not a stored/authoritative field, consistent
  with W4G1's "signals are stored, verdicts are derived" philosophy applied to
  URL identity rather than trust).
- **`Index`/`Log` handlers**: xSAVIKx's `AppendLogEntry` date-grouping algorithm
  (newest day first, newest entry within a day first, caller-supplied date) is
  a solid concrete reference for our `Log` writer. For `Index`, W4G1's
  conformance rule (§8: subdirectory `index.md` must not carry frontmatter,
  root `index.md` must declare `okf_version`) is a check worth reusing verbatim
  in `Conformance()`.
- **Error handling**: both repos use plain typed errors (Rust enums / would-be
  Go sentinel errors), not opaque/dynamic ones — validates using a small closed
  set of Go error types (or wrapped sentinel errors) for `Concept`/`Bundle`
  parse failures, matching Go idiom and W4G1's approach directly.
- **Reject carrying over**: no v0.2 trust/provenance/attestation surface: don't
  add `sources`/trust-tier/staleness handling; out of scope per v0.1 target.
  No CLI subcommand surface baked into the library; that's an application
  concern layered on top, not part of the library contract.
