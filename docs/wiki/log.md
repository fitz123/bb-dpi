# Wiki log

Chronological, append-only record of wiki events (ingests, queries,
lint passes). Newest at the bottom.

Entry format: `## [YYYY-MM-DD] <type> | <subject>` followed by a 1-3
line summary of what changed.

`grep "^## \[" log.md | tail -10` gives the recent timeline.

---

## [2026-05-14] bootstrap | DPI-evasion research wiki created

Wiki structure created at `docs/wiki/`. Three-layer pattern per
Karpathy's LLM Wiki gist: raw sources / wiki / schema. Six concept
pages seeded (REALITY protocol, ASN-match SNI camouflage, DPI flow-
learning, XHTTP transport, two-sided tcpdump diagnostic, uTLS
fingerprint staleness). Three source pages ingested from project
memory (RU DPI behavior, chain-relay rationale, two-sided tcpdump
methodology). One cross-source synthesis page: "2026-05 RU DPI
snapshot". Index + schema in place.

PII rules applied throughout — concrete fleet identifiers (server
names, IPs, AS numbers when tied to specific operators, real SNI
hostnames) held back to the operator's gitignored `servers.json` and
out-of-repo memory. Wiki content is the *generalisable knowledge*
extracted from those sources, not the fleet inventory itself.

Index has a "concepts referenced but not yet a page" section listing
candidates for future ingestion as their second reference appears.

## [2026-05-14] lint | scrub PR-#13 remnants

Pre-PR dual-review (codex + Opus) flagged 23 wiki-links to a phantom
`[[s-arch-decisions]]` source page and 4 references to a non-existent
`docs/architecture-decisions.md` (the closed PR-#13 register). All
references purged: 23 `[[s-arch-decisions]]` citations removed
in-place (the surrounding prose already carried each claim);
`s-arch-decisions` source row dropped from `index.md`; SCHEMA.md
cross-reference example and frontmatter example updated to a real
page; SCHEMA.md PII section made self-contained (no longer anchored
to an external register); README.md and `log.md` cleaned of stale
references; AGENTS.md "canonical decision register" claim softened
to match what the wiki actually contains. Concept-page Sources
sections repointed at real sources (memory pages, xray-core source,
upstream docs). Also: codex flagged an XHTTP `mode:auto` overclaim —
rewritten to match the documented behavior (stream-up for TLS-H2,
stream-one for REALITY without `downloadSettings`, packet-up
otherwise; no adaptive middlebox switching) with upstream citation.
Empty `docs/wiki/project/` directory removed.
