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
fingerprint staleness). Four source pages ingested (the new
consolidated architecture-decisions register + three project-memory
files: RU DPI behavior, chain-relay rationale, two-sided tcpdump
methodology). One cross-source synthesis page: "2026-05 RU DPI
snapshot". Index + schema in place.

PII rules applied throughout — concrete fleet identifiers (server
names, IPs, AS numbers when tied to specific operators, real SNI
hostnames) held back to the operator's gitignored `servers.json` and
out-of-repo memory. Wiki content is the *generalisable knowledge*
extracted from those sources, not the fleet inventory itself.

Index has a "concepts referenced but not yet a page" section listing
candidates for future ingestion as their second reference appears.
