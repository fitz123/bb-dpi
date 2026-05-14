# Wiki schema — how to maintain this wiki

This file is the contract between you (a future LLM agent session) and
the wiki. Read it before doing anything in `docs/wiki/`.

## What this wiki is

A persistent, LLM-maintained knowledge base for **DPI evasion research**:
the protocols (REALITY, VLESS, XHTTP, TCP+vision, MPTCP, ECH, etc.), the
documented adversary behaviors (TSPU and similar in-path DPI),
diagnostic techniques, and the cross-source synthesis tying it all
together. The wiki sits alongside the `bb-dpi` reference implementation
but its subject is the *broader topic*, not the specific fleet.

Pattern source: [Karpathy — LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Three layers:

1. **Raw sources** — immutable inputs (memory files, plan docs, ADRs,
   external articles, papers). Stored at:
   - In-repo: `docs/plans/`, `docs/adr-NNN-*.md`, `docs/architecture-decisions.md`
   - Out-of-repo: `~/.claude/projects/-Users-ninja-bb-dpi/memory/`
     (private; never link to specific paths from in-repo wiki pages)
   - External: cite by URL
2. **The wiki** (this directory) — markdown pages the agent writes
   and maintains. Owned entirely by the agent.
3. **The schema** — this file.

## Layout

```
docs/wiki/
├── SCHEMA.md          ← this file
├── README.md          ← front door for humans
├── index.md           ← catalog of all pages, organized by category
├── log.md             ← append-only chronological event log
├── concepts/          ← protocol/technique/adversary-behavior pages
├── sources/           ← per-source summary pages (one per ingested doc)
└── synthesis/         ← cross-source insight pages
```

Page filenames: `kebab-case-no-spaces.md`. Title in the first line as
`# Title`.

## Conventions

### Cross-references

Wiki links use Obsidian-style double-brackets so they work both
as plain text and (optionally) as Obsidian/MkDocs links:

```
[[reality-protocol]]      → links to concepts/reality-protocol.md
[[s-arch-decisions]]      → links to sources/s-arch-decisions.md
```

Within a page, anchor to specific section headings as
`[[reality-protocol#mirror-conn-forwarding]]`.

External citations use standard markdown links:
```
[Karpathy gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)
```

### Page frontmatter (optional)

YAML frontmatter for searchability — use when the metadata adds signal:

```yaml
---
tags: [reality, sni, camouflage]
sources: [s-arch-decisions, s-memory-chain-relay-rationale]
updated: 2026-05-14
---
```

Skip frontmatter on synthesis/log pages where it adds nothing.

### Length and tone

- **Concept pages**: 50-150 lines. Open with a one-paragraph definition,
  then mechanism, then observed-in-practice notes, then mitigations and
  open questions. Cite sources inline.
- **Source pages**: 20-60 lines. Title = source title. Body = key
  takeaways extracted from the source, linked to concept pages.
- **Synthesis pages**: as long as needed. These are the
  "what-have-we-learned-overall" pages that compound value over time.

Tight, factual, citation-anchored. No marketing-style prose.

## PII rules — load-bearing

This wiki lives in a public-ish git repo with a CI gitleaks scan. The
project's standing PII rules apply (see
`docs/architecture-decisions.md` §6.12):

**Forbidden in committed wiki content:**
- Specific server IP literals from the operator's fleet
- Specific server names (the fleet's internal hostnames — kept in
  out-of-repo memory, never repeat in-repo)
- Specific hosting provider proper nouns when tied to a specific
  fleet member (provider names in general adversary-research context
  are OK if they're documented adversary or victim categories)
- Specific real SNI hostnames the operator's fleet uses for camouflage
- Home-directory paths (use `~/` form)
- Real personal/corp identities

**Allowed:**
- Documented adversary actors named in public security research
  (e.g., "TSPU" as a documented entity)
- Public protocol names (REALITY, VLESS, XHTTP, etc.)
- Generic geographic terms when load-bearing for the research (a wiki
  about RU DPI must say "RU DPI" somewhere)
- ASN numbers in abstract examples ("a relay on AS49505 with SNI on
  the same AS") — but not paired with the operator's specific server
- External URLs cited as sources

When in doubt, generalize: "the operator's relay" instead of a server
name, "a CDN-CNAMED hostname" instead of a literal camouflage SNI.
Concrete values live in the operator's gitignored `servers.json` and
out-of-repo memory.

## Operations

### Ingest

When the operator adds a new source (or asks you to ingest an existing
in-repo doc):

1. Read the full source.
2. Brief discussion with the operator on key takeaways (skip if the
   source is small and the takeaways are obvious).
3. Create `sources/s-<slug>.md` with the source's key facts +
   wiki-links to the concept pages it touches.
4. For each concept the source touches:
   - If a concept page exists, update it (new facts, contradiction
     flags, updated synthesis).
   - If not, create it.
5. Update `index.md` (add the new source page; add any new concept
   pages).
6. Append a log entry: `## [YYYY-MM-DD] ingest | <source title>`
   followed by a 1-3 line summary of what changed.

### Query

When the operator asks a question against the wiki:

1. Read `index.md` first to find relevant pages.
2. Drill into the relevant pages + their cross-references.
3. Synthesize the answer with `[[wiki-link]]` citations to the pages
   you used.
4. If the synthesis surfaces a new insight worth keeping, ask the
   operator if you should file it as a `synthesis/` page.
5. Append a log entry if the query did substantive work.

### Lint

Periodic health check (operator triggers explicitly):

1. Read `index.md` and a sampling of pages.
2. Look for: contradictions across pages, stale claims (newer sources
   superseding old ones), orphan pages (no inbound links), concepts
   mentioned in multiple pages without a dedicated concept page,
   missing cross-references.
3. Report findings as a markdown checklist for the operator to triage.
4. Apply approved fixes; append log entry summarizing what was fixed.

## Bootstrap state (as of 2026-05-14)

This wiki is freshly seeded from in-repo project knowledge:

- `docs/architecture-decisions.md` (the consolidated decision register)
- Memory files from the project's local memory directory (DPI-research-
  relevant entries; project-internal workflow rules excluded since they
  don't transfer to the broader research domain)
- The `docs/plans/` rollout plans
- ADR-001

Seed scope is deliberately small: ~6 concept pages, ~4 source pages,
1 synthesis page. Expand by ingesting external sources as the
operator brings them in.

## Tooling

- **Plain markdown** + git. No special build step.
- **Search**: at this scale, `grep -ri` over the wiki + the index page
  is enough. If the wiki grows past ~50 pages, consider [qmd](https://github.com/tobi/qmd)
  for hybrid BM25/vector search.
- **Optional**: Obsidian for graph view; the wiki structure is
  Obsidian-compatible (`[[wiki-link]]` syntax, frontmatter).

## When this schema needs to change

Co-evolve it as you learn what works. If you find yourself working
around the schema rather than with it, propose an edit to the operator
and update both this file and any pages it affects.
