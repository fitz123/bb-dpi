# DPI Evasion Research Wiki

A persistent, LLM-maintained knowledge base for the broader subject
this project lives in: **DPI evasion protocols, observed adversary
behaviors, and the diagnostic toolbox**.

## Quick navigation

- **[Schema](./SCHEMA.md)** — how the wiki is structured and how to
  add to it (read first if you're an LLM session).
- **[Index](./index.md)** — catalog of every page, by category.
- **[Log](./log.md)** — chronological record of ingests, queries, and
  lint passes.
- `concepts/` — protocol / technique / adversary-behavior pages.
- `sources/` — per-source summary pages.
- `synthesis/` — multi-source insight pages.

## What this wiki is NOT

- **Not the `bb-dpi` reference implementation**. The implementation
  details live in `../architecture-decisions.md`, `../adr-*.md`, the
  scripts, and the configs. This wiki is the *generalisable knowledge*
  about the subject domain.
- **Not RAG**. There's no embedding store, no chunking. It's plain
  markdown that an LLM reads end-to-end and maintains by hand. At ~50
  pages the index file is the search engine.

## Pattern source

Karpathy's [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Three-layer pattern: raw sources → wiki → schema.

## Conventions at a glance

- Page filenames: `kebab-case.md` in the appropriate subdirectory.
- Cross-references: `[[wiki-link]]` Obsidian-style.
- Source pages: prefix `s-` for distinguishability.
- PII scrubbed per [`architecture-decisions.md` §6.12](../architecture-decisions.md).
  This is non-negotiable — CI gitleaks blocks otherwise.

Full conventions in [SCHEMA.md](./SCHEMA.md).
