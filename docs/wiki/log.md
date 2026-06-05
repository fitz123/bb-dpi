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

## [2026-05-17] ingest | rkn-block-checker — community DPI diagnostic tool

Ingested [github.com/MayersScott/rkn-block-checker](https://github.com/MayersScott/rkn-block-checker)
as [[s-tool-rkn-block-checker]] — Python CLI (MIT, on PyPI) that
probes a RU consumer vantage layer-by-layer and classifies failures
by signal type (TLS-DPI, DNS-poison, HTTP-stub, TCP-RST).
Measurement-grounded: classifiers tied to observable wire patterns
(RST timing after ClientHello, sys-vs-DoH address-set disjoint),
not operator claim. Sits higher on the evidence ladder than
deepwiki/rcd27 or Habr QnA forum threads.

Captured nuance: tool's README states "IPv4 only. Some Russian ISPs
treat IPv6 differently (often less filtered)." This is a second
operator-claim data point on v6-vs-v4, pointing OPPOSITE direction
to deepwiki/rcd27's "Mar 2026 v6 parity reached" claim. Wiki now
preserves both single-source claims as a documented disagreement
rather than picking a winner. The procurement conclusion (don't
buy v6 for DPI bypass) stands because the load-bearing leg is
[[s-2026-05-xray-relay-community-reports]]' absence-of-positive-
evidence, not the parity claim.

Concept-page updates:
- [[two-sided-tcpdump-diagnostic]] — added "Before reaching for
  tcpdump" section pointing to rkn-check as the lighter first-
  triage tool.
- [[asn-match-sni-camouflage]] — added "Validation tool" section
  noting rkn-check as a candidate-hostname reachability pre-check
  (necessary not sufficient — tool connects to the hostname's real
  IP, not to the relay IP with SNI override). End-to-end ASN-match
  validation still requires `openssl s_client -connect <relay-ip>:443
  -servername <candidate>`.
- [[dpi-flow-learning]] — Sources updated to include rkn-check as a
  handshake-stage triage tool. If rkn-check returns `✓ OK` but a
  tunnel is dead, flow-burn is the next hypothesis to test;
  rkn-check cannot positively identify flow-burn (different signal
  class — established-tunnel payload drop vs handshake stage).

Installed on the operator's RU consumer vantage Mac via uv tool
install; baseline snapshot captured to `~/rkn-monitor/` for longitudinal
tracking. The baseline confirmed VPN-on state (15/15 RKN-restricted
sites pass), validating the current chain is operating as intended.

## [2026-05-17] concept | dns-aaaa-cascade-failure — hypothesis page

> *Superseded by the [2026-05-17] correction entry below. The page
> was moved from `concepts/` to `synthesis/` and the AAAA-cascade
> mechanism was most-likely invalidated by a post-hoc routing check.
> Read the correction entry below before relying on anything in
> this entry's framing.*


First-party investigation of a [[s-tool-rkn-block-checker]] `✗ DNS`
verdict on `www.vtb.ru` from a RU consumer vantage (VPN-off baseline)
produced a candidate mechanism for the "site won't open in browser
but `dig` works" symptom class. Documented as new concept page
[[dns-aaaa-cascade-failure]] — **filed as hypothesis, not mechanism**.

Hypothesised cause: macOS libc `getaddrinfo(host, family=AF_INET)`
may issue parallel AAAA queries against the configured resolver chain.
If an adversary (e.g., TSPU) drops AAAA on a public-DNS path that
macOS picks, the AAAA timeout could cascade into a full `getaddrinfo`
failure. `dig` sends explicit A-only and would not be affected.

Single-observation evidence (2026-05-17, RU vantage, VPN-off):
`dig` returns IP fast; `dig @8.8.8.8 AAAA www.vtb.ru` timed out once,
then returned cleanly on retries; `getaddrinfo(www.vtb.ru, AF_INET)`
raised `gaierror`; `curl` failed "Could not resolve"; `dscacheutil`
flush did not recover.

**Disconfirming follow-up sweep**: five RU banking/gov hosts queried
for `AAAA @8.8.8.8` minutes later returned cleanly across the board,
yet `getaddrinfo(www.vtb.ru, AF_INET)` continued to fail in the same
shell. AAAA-timeout-as-active-mechanism is inconsistent with the
followup; alternative hypotheses (mDNSResponder negative cache stickier
than dscacheutil flush; scoped resolvers; DNSSEC; profile-installed
DoH) are documented on the page as competing explanations.

What's confirmed vs not:
- *Confirmed*: the diagnostic asymmetry (dig works, libc apps don't)
  for `www.vtb.ru` from this vantage at this time.
- *Not confirmed*: AAAA-cascade as the active mechanism; TSPU as the
  attacker; probabilistic targeting of banking hosts.

Concept touches:
- [[s-tool-rkn-block-checker]] — reference to the new concept as the
  candidate mechanism behind one possible cause of the `✗ DNS`
  verdict; the source page also narrowed its ASN-match-validation
  claim to "necessary, not sufficient" (rkn-check connects to target
  hostname, not relay IP with SNI override).
- [[asn-match-sni-camouflage]] — Validation tool section updated to
  reflect the narrower rkn-check claim plus the openssl-against-relay
  end-to-end check.
- [[two-sided-tcpdump-diagnostic]] — Before-reaching-for-tcpdump
  section clarified: rkn-check is a handshake-stage classifier; flow-
  burn (established-tunnel payload drop) is NOT what rkn-check sees,
  it's what two-sided tcpdump diagnoses.

The earlier "AAAA-cascade is the mechanism" framing in this log
existed in the initial commit; the rewrite was prompted by a
dual-review round (Codex+Opus) that flagged the overclaim and the
omission of the disconfirming followup data.

## [2026-05-17] correction | dns-aaaa-cascade-failure — hypothesis most-likely invalidated

A follow-up baseline-routing verification on the same RU vantage
Mac on 2026-05-17 most-likely invalidated the dns-aaaa-cascade-
failure mechanism hypothesis. The initial concept page was written
under the implicit assumption that the libc resolver was talking
directly to `8.8.8.8` (VPN TUN off). The routing check showed
otherwise:

- `pgrep -fl sing-box` returned a running sing-box process at
  correction time.
- `route -n get 1.1.1.1` and `route -n get 8.8.8.8` both pointed at
  `utun7` (sing-box TUN), showing auto-route was active **at
  correction time** — consistent with continuous interception during
  the original observation window if no route flap occurred.
- Re-verification: `getaddrinfo("www.vtb.ru", AF_INET)` returned
  cleanly 5/5; `rkn-check` verdict flipped to `✓ OK`; sing-box log
  showed clean DNS exchange.

Conditional on continuous TUN auto-route through the observation
window (an inference from the post-hoc snapshot, not a directly
proven continuity), the proposed mechanism is structurally
inaccessible: traffic never reached `8.8.8.8`, so "TSPU drops AAAA
on 8.8.8.8" cannot have been the active cause. The transient
gaierror is real but unattributed; no confirmed mechanism remains.

[[dns-aaaa-cascade-failure]] rewritten as **most-likely-invalidated
hypothesis**, preserved as a teaching case for the methodology
mistake (don't codify a single observation as a named concept;
check baseline routing first). Other pages should not link to it
as a citable mechanism. Page lead and Status section explicitly
say so.

Touched:
- `synthesis/dns-aaaa-cascade-failure.md` (was `concepts/dns-aaaa-cascade-failure.md`
  before the round-1 dual-review fix moved it; slug preserved so
  inbound links continue to resolve) — complete rewrite. Lead
  paragraph + Status section + "Why the original mechanism is
  most-likely invalidated" section make the status explicit. New
  "Lessons" section captures the methodology mistake. Operational
  guidance section retained (independent of the most-likely-
  invalidated mechanism) for future symptoms in the same class.
- `index.md` — row rewritten: "Invalidated hypothesis ... kept as a
  teaching case ... other pages should not link to this as a citable
  mechanism."
- `sources/s-tool-rkn-block-checker.md` — the Touched-concept-pages
  bullet for dns-aaaa updated: the tool's `✗ DNS` classifier was
  correct in flagging DNS-layer failure; the proposed mechanism was
  wrong.

Lessons captured into the operator's project memory: the search-
all-surfaces rule for overclaim propagation (file
`feedback_wiki_overclaim_propagation.md`) and the new baseline-
routing-before-attribution rule (file
`feedback_verify_baseline_routing_first.md`). Memory paths
intentionally not wiki-linked — they're out-of-repo per schema.

Page moved from `concepts/` to `synthesis/` (keeping the slug
stable so inbound links continue to resolve) per the schema's
distinction between "concept = protocol/technique/adversary-
behavior page" and "synthesis = cross-source insight / open-
question collection". A single-observation hypothesis whose
mechanism was most-likely invalidated, kept as a teaching case,
fits synthesis, not concepts.

## [2026-05-22] ingest | 2026-05 multi-reviewer audit of a RU-VDS shortlist

A single-researcher shortlist of RU VDS providers for a second
RU-domestic chain-relay (sibling exercise to
[[s-2026-05-ru-vps-ipv6-procurement-scan]]) was put through an
adversarial multi-reviewer pass: Codex via `codex exec`, an Opus
subagent via `Task`, and Gemini via `gemini --approval-mode plan`,
parallel dispatch, JSON output schema. Three independent reviewers
converged on enough structural defects to invalidate the lead's
rank order.

New pages:
- [[s-2026-05-multi-reviewer-vds-shortlist]] — source page with the
  verdicts, convergent findings, alternatives proposed, and anti-
  candidates added.
- [[2026-05-ru-vds-shortlist-multi-review]] — synthesis with the
  re-ranked shortlist (DataCheap, AdminVPS, HOSTKEY, JustHost
  far-east, RUVDS conditional), expanded anti-candidate list, a
  mandatory pre-purchase verification checklist, and methodology
  lessons.

Concept page update:
- [[hosting-provider-as-dpi-variable]] — added a "Procurement
  verification" section (axis-independent meta-discipline:
  RDAP-of-allocated-IP, vendor-payment-page read, ToS scan, DC
  building cross-reference, two-source ASN agreement) and a
  "Cover-site narrative — what's load-bearing and what isn't"
  section that downgrades the "match cover to AS-tenant
  archetype" heuristic in favour of camouflage diversity +
  cert/SNI coherence.

Convergent findings worth highlighting:

- **AS-to-provider attribution is unstable in the RU VDS market.**
  Reviewers disagreed on the authoritative ASN for at least three
  brand names (VDSina, FirstByte, JustHost). A self-contained
  internal contradiction in the lead's report (AS197695 listed as
  both RUVDS's AS and REG.RU's AS in the exclusions) was the
  cheapest signal that an audit was needed. Vendor marketing is
  not ground truth; RDAP of the allocated IP at provisioning time
  is the only resolution.
- **"Crypto-friendly" headline claims overstate.** Three of the
  five lead-shortlisted providers had no native crypto per their
  own checkout pages; a fourth had aggregator-gated crypto with
  ~30%+ effective fees per community reports.
- **FirstByte (AS204997) demoted to anti-candidate.** PayPal-only
  payment in a no-PayPal jurisdiction (since Mar 2022) + UK-shell
  over RU-ops pattern-matches the structures sanctioned in the
  Nov 2025 US/UK/AU coordinated wave (Media Land) and the Jul 2025
  Aeza wave.
- **SPb-PITER-IX overlap is a real fate-sharing axis.** Beget at
  PITER-IX is less isolated from a Selectel-SPb relay than the
  own-ASN headline implies. Geographic diversity (Kazan /
  Novosibirsk / Irkutsk / Khabarovsk PoPs) is the cleaner
  orthogonality lever.
- **DC overlap at L1** (DataPro Moscow housing VDSina + HOSTKEY +
  ProfitServer) is a separate fate-sharing axis from the AS.
  Different ASNs in the same physical facility ≠ fate-isolated.

PII discipline: research framed around generic procurement for an
abstract "second RU-domestic relay" given an existing relay on
Selectel AS49505. No fleet-specific server names or IPs. Provider
names appear as research subjects, consistent with the convention
already established by [[hosting-provider-as-dpi-variable]] and
[[s-2026-05-ru-vps-ipv6-procurement-scan]].

Methodology lesson captured into the synthesis page: adversarial
reviewers find divergent things (Codex was best at vendor-page
fact-checking; Opus was best at L1 fate-sharing + corporate-
structure pattern matching; Gemini was best at proposing
geographically unconventional alternatives). The convergence comes
from the union of distinct attack patterns. A single reviewer
would have missed at least one axis.

## [2026-05-22] validation | second RU relay candidate pre-purchase verification + consumer-vantage rkn-check

Post-multi-review focused validation pass against checks 2-5 of the
pre-purchase verification checklist (Check 1 = RDAP-of-allocated-IP
deferred to provisioning). Plus a consumer-vantage rkn-check probe
from an operator RU Mac (VPN-off) against the surviving candidates'
marketing domains to test for AS-level TSPU pre-blocking.

Validation surfaced structural errors the multi-review missed:

- **AdminVPS (AS211183) transits via REG.RU (AS197695)** as upstream.
  Same fate-sharing defect that disqualifies RUVDS, but at the
  transit-upstream layer rather than the announce-from-AS layer.
  Multi-review's "two-source ASN agreement" check would not catch
  this because the announce-from AS was correct.
- **HOSTKEY ToS explicitly bans the workload** (public proxy /
  VPN-as-a-service prohibited; VPN servers including personal-use
  require prior consent; Tor outbound prohibited). BitPay crypto
  plus is real but moot when the operational workload itself is
  ToS-forbidden.
- **RUVDS confirmed: AS197695 = REG.RU**, RUVDS has no separately
  announced AS, and its ToS explicitly forbids "means to obtain
  access to resources with restricted access in the Russian
  Federation" plus private VPN >10 GB/day. Three independent
  disqualifiers stack.

Multi-review claim corrections:

- **AS207651 = VDSina, not JustHost.** Resolves the Opus-vs-Gemini
  disagreement: Gemini was right on AS51659 (`ASBAXET / LLC Baxet`).
  Opus's AS207651 was actually VDSina's, already on the L1-overlap
  exclusion list.
- **AS59729 ≠ AdminVPS** — that's Bulgarian "ITL-BG", unrelated.
- **DataCheap is Moscow-only** — the multi-review's "Kazan +
  Novosibirsk PoPs" claim was not corroborated by DataCheap's own
  About page. AS16262 announces multi-city prefixes but the
  physical operation is single-DC at ul. Ugreshskaya in south
  Moscow.
- **JustHost Khabarovsk PoP not corroborated.** Novosibirsk + Kazan
  confirmed; Khabarovsk was a multi-review hypothesis that did not
  hold.

Consumer-vantage rkn-check probe from an operator RU Mac (VPN-off):

- Bundled sweep returned "Likely in an RKN-blocked zone (medium
  confidence)" — whitelist 20/21, blacklist 0/15 open with 12
  TLS-DPI silent drops + 2 timeouts. Vantage genuineness confirmed.
- AS16262 (DataCheap → datacheap.ru): `✓ OK`, TCP 37ms, TLS 85ms,
  PLT 170ms, no DPI flags.
- AS51659 (Baxet → justhost.ru): `✓ OK`, TCP 59ms, TLS 126ms,
  PLT 321ms, no DPI flags.

Neither AS is on TSPU's elevated-CIDR set at the consumer vantage.
Caveat per [[s-tool-rkn-block-checker]]: this is the *necessary*
AS-reachability check, not the *sufficient* end-to-end ASN-match-SNI
validation — that happens at provisioning with `openssl s_client`
against the allocated relay IP with `-servername <chosen-SNI>`.

Final post-validation buy order:

1. **DataCheap (AS16262, Moscow)** — buy first; the only candidate
   with no structural defects. Moscow-only and SBP-only payment are
   the accepted trades.
2. **JustHost / Baxet (AS51659, Novosibirsk PoP)** — buy second for
   genuine geographic orthogonality; ToS scan at signup is the
   remaining gate.
3. Stop. The other shortlist members are out structurally.

Touched pages:
- [[2026-05-ru-vds-shortlist-multi-review]] — appended a "Validation
  pass (2026-05-22)" section with the resolved questions, new
  disqualifiers, corrections to multi-review claims, the
  consumer-vantage rkn-check table, and the final post-validation
  go/no-go.
- [[hosting-provider-as-dpi-variable]] — Procurement verification
  checklist extended with a 6th step (upstream-transit chain
  inspection). The AdminVPS-through-REG.RU finding showed that an
  own-AS candidate can still be fate-shared at the upstream-transit
  layer — a separate axis from announce-from-AS attribution.

PII discipline: no fleet-specific identifiers introduced. The
operator's RU Mac is referenced as "an operator RU consumer Mac"
rather than by name; the candidate domains (datacheap.ru,
justhost.ru) are public vendor marketing sites probed for
AS-reachability, not the operator's camouflage SNIs.
