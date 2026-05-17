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

## [2026-05-17] correction | dns-aaaa-cascade-failure — hypothesis invalidated

A follow-up baseline-routing verification on the same RU vantage
Mac on 2026-05-17 invalidated the dns-aaaa-cascade-failure mechanism
hypothesis. The initial concept page was written under the implicit
assumption that the libc resolver was talking directly to `8.8.8.8`
(VPN TUN off). Verification of the actual routing showed otherwise:

- `pgrep -fl sing-box` returned PID 45707, sing-box was running.
- `route -n get 1.1.1.1` and `route -n get 8.8.8.8` both pointed at
  `utun7` (sing-box TUN), proving auto-route was intercepting system
  DNS at the time the original gaierror was observed.
- Re-verification of `getaddrinfo("www.vtb.ru", AF_INET)` returned
  cleanly 5/5 times; `rkn-check` verdict flipped to `✓ OK`; sing-box
  log showed clean DNS exchange.

That breaks the proposed mechanism end-to-end: traffic never reached
`8.8.8.8`, so "TSPU drops AAAA on 8.8.8.8" can't have been the
active cause. The transient gaierror is real but unattributed; no
confirmed mechanism remains.

[[dns-aaaa-cascade-failure]] rewritten as **invalidated hypothesis**,
preserved as a teaching case for the methodology mistake (don't
codify a single observation as a named concept; check baseline
routing first). Other pages should not link to it as a citable
mechanism. Status section explicitly says so.

Touched:
- `concepts/dns-aaaa-cascade-failure.md` — complete rewrite. Lead
  paragraph + "Why the original mechanism hypothesis is invalidated"
  section make the status explicit. New "Lessons" section captures
  the methodology mistake. Operational guidance section retained
  (independent of the invalidated mechanism) for future symptoms
  in the same class.
- `index.md` — row rewritten: "Invalidated hypothesis ... kept as a
  teaching case ... other pages should not link to this as a citable
  mechanism."
- `sources/s-tool-rkn-block-checker.md` — the Touched-concept-pages
  bullet for dns-aaaa updated: the tool's `✗ DNS` classifier was
  correct in flagging DNS-layer failure; the proposed mechanism was
  wrong.

Lessons captured into project memory as
[[feedback-wiki-overclaim-propagation]] (covers the propagation
issue from round-2) and a methodology note: always verify baseline
routing before attributing a symptom to a named mechanism.
