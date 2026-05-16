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
source page (slug previously catalogued as the consolidated decision
register) and 4 references to its non-existent in-repo doc (PR #13
was closed without merging). All references purged: 23 citations
removed in-place (the surrounding prose already carried each claim);
the phantom-source row dropped from `index.md`; SCHEMA's
cross-reference example and frontmatter example updated to a real
page; SCHEMA's PII section made self-contained (no longer anchored
to an external register); README and this log cleaned of stale
references; AGENTS.md "canonical decision register" claim softened
to match what the wiki actually contains. Concept-page Sources
sections repointed at real sources (memory pages, xray-core source,
upstream docs). Also: codex flagged an XHTTP `mode:auto` overclaim —
rewritten to match the documented behavior (stream-up for TLS-H2,
stream-one for REALITY without `downloadSettings`, packet-up
otherwise; no adaptive middlebox switching) with upstream citation.
Empty `docs/wiki/project/` directory removed.

## [2026-05-16] ingest | 2026-05 RU VPS / IPv6 / TSPU research wave

Four external research sources ingested from a parallel multi-agent
web research wave triggered by operator's question "is IPv6 a viable
DPI-evasion variable for a second RU chain-relay procurement":

- [[s-2026-05-ru-vps-ipv6-procurement-scan]] — May 2026 RU VPS
  market scan for native dual-stack v4+v6.
- [[s-2026-05-ipv6-bgp-path-aws-stockholm]] — v6 BGP path quality
  from RU hosters to AWS eu-north-1. Surfaces the v6-as-bypass
  refutation (TSPU IPv6 parity Mar 2026).
- [[s-2026-05-xray-relay-community-reports]] — 2025-2026 community
  consensus + dated DPI events timeline (15-20 KB freeze Jun 2025,
  Aeza purge Dec 2025, AI-keyed REALITY detector Dec 2025, 439 VPN
  services blocked Jan 2026, mobile whitelist Apr 2026).
- [[s-2026-05-tspu-asn-camouflage-research]] — measurement-grounded
  TSPU fingerprinting (SNI/IP correspondence, port-443 bias, empty-
  SNI exemption); per-provider compliance posture; pending Apr 2026
  hosting-as-controller legislation; YC sub-AS-granularity contradiction
  flagged.

New concept page: [[hosting-provider-as-dpi-variable]] — captures
the three-axis framework (source-ASN↔SNI pairing, compliance
posture, TSPU inspection profile of the source ASN). Aeza
disqualified from the candidate set (OFAC Jul 2025 + active customer
purge Dec 2025) — documented with multi-source citations.

Concept-page updates:
- [[dpi-flow-learning]] — adjacent-heuristics section added
  (15-20 KB freeze, CIDR whitelist, port-443 bias, empty-SNI, v6
  parity).
- [[asn-match-sni-camouflage]] — SNI/IP-correspondence mechanism
  documented; sub-AS granularity refinement (Yandex Cloud's two
  ASNs filtered separately by TSPU).
- [[reality-protocol]] — port-443 selection-bias and AI-keyed
  REALITY detector (Dec 2025) added; xray-core 25.12.8+
  countermeasures noted.

Synthesis page [[2026-05-ru-dpi-snapshot]] expanded with the
public-research observed table and the v6-bypass-refutation
section.

PII discipline: research is about generic procurement candidates
and TSPU-as-research-subject. No fleet-specific server names,
IPs, or camouflage SNIs introduced. Selectel named as a research
subject (existing memory and earlier wiki pages already
reference it as a provider, not as a fleet identifier).

Operator-facing headline preserved in the wiki: the v6-as-bypass
hypothesis is unsupported by 2026 public research (two independent
sources). v6 may still be procured for opportunistic latency
benefits but should not be the primary procurement rationale.
