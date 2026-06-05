# Synthesis — 2026-05 RU VDS shortlist after multi-reviewer audit

A single-researcher candidate shortlist for an additional RU-domestic
chain-relay was put through an adversarial multi-reviewer pass (Codex
+ Opus + Gemini). The reviewers converged on enough structural
defects that the lead's rank order does not survive. This page
records the post-review shortlist plus the methodology lessons.

For the raw reviewer outputs and convergence analysis see
[[s-2026-05-multi-reviewer-vds-shortlist]].

## TL;DR

- The "leading" RU-VDS pick from a single agent's first-pass research
  (RUVDS) was knocked off the top by an internal contradiction that
  three reviewers all noticed: the same researcher had marked
  AS197695 as REG.RU's ASN in its own exclusions but then used
  AS197695 as the AS of its #1 pick. The 2026-05-22 validation pass
  settled it against RUVDS — AS197695 *is* REG.RU's AS and RUVDS has
  no separately announced AS, so it is fate-shared with an
  explicitly-excluded provider, not independently rankable. (The
  first-pass framing had treated this as open, settle-by-RDAP, not by
  vendor marketing; the validation pass is that resolution.)
- **The "vendor says they take crypto" claim is unreliable.** Three
  of the five lead-shortlisted providers had no native crypto
  payment per their official pages (Beget, Timeweb, FirstByte); a
  fourth (VDSina) has aggregator-gated crypto with ~30%+ effective
  fees per community reports.
- **SPb / Moscow IX overlap is a meaningful fate-sharing axis** even
  for nominally-independent providers. Beget at PITER-IX is not as
  well-isolated from a Selectel-SPb relay as own-ASN headlines
  imply. Geographic diversity (Kazan / Novosibirsk / Irkutsk /
  Khabarovsk PoPs) is the cleaner orthogonality lever.
- **Physical DC overlap at L1 is its own axis.** DataPro Moscow
  houses VDSina, HOSTKEY, and several other RU hosts at the
  building/power/upstream-fiber level. Different ASNs in the same
  DC ≠ fate-isolated.
- **The "match cover-site to provider's typical-tenant archetype"
  heuristic is weak.** Provider tenant mixes are wide; a generic
  small-business / docs / portfolio / CMS page is plausible on
  essentially any candidate AS. Camouflage diversity (don't clone
  the existing cover) is load-bearing; AS-sociology is not.

## Post-review re-ranking

> **Snapshot — pre-validation.** This table reflects the
> post-multi-review state *before* the validation pass below. Several
> entries were later corrected (DataCheap is Moscow-only, not
> Kazan/Novosibirsk; JustHost = AS51659; AdminVPS transits REG.RU; the
> AS207651/AS59729 attributions were wrong). For the current verdicts
> see "Final post-validation go/no-go".

The lead's order was: RUVDS, Beget, Timeweb, VDSina, FirstByte. The
re-ranked shortlist:

| # | Provider | ASN | Cities | Strongest argument | Strongest catch |
|---|---|---|---|---|---|
| 1 | DataCheap (Delta) | AS16262 | Moscow + Kazan + Novosibirsk | Own Tier-III DC + own ASN + long history + geographic options | SBP-only payment (no native crypto) |
| 2 | AdminVPS | AS211183 / AS59729 (varies) | Moscow + multi-region | Three-reviewer convergence; mainstream tenant blend | Crypto path ambiguous, needs checkout verification |
| 3 | HOSTKEY | AS57043 | Moscow (DataPro) | Foreign legal vehicle = jurisdictional orthogonality; BitPay crypto | DataPro L1 overlap with VDSina; AS57043 has visible hosting/Tor profile |
| 4 | JustHost (Baxet) far-east PoP | AS51659 / AS207651 — reviewers disagreed | Novosibirsk / Kazan / Khabarovsk | Maximum geographic / IXP isolation from SPb-Moscow core | ASN attribution unstable; needs RDAP at allocation |
| 5 | RUVDS (conditional) | AS197695 — disputed | Moscow (Korolev) | Own DC, established brand | Probably on REG.RU's AS infrastructure; if so, fate-sharing with an explicitly-excluded provider |
| — | FirstByte | AS204997 | Moscow / UK shell | (removed) | PayPal-only payment + UK-shell-over-RU-ops = sanctioned-pattern; do not use |

## Anti-candidates added by the review

In addition to the pre-review excludes ([[s-2026-05-ru-vps-ipv6-procurement-scan]]
and [[s-2026-05-tspu-asn-camouflage-research]] cover Aeza,
PQ.hosting, 4vps.su, Reg.ru, Selectel), this round adds:

- **FirstByte** — broken payment + sanctioned-pattern structure
- **Inferno Solutions (AS200487)** — Stockholm court seized servers
  for TA505 hosting (proposed by Gemini, rejected by Opus on
  exactly that ground)
- **Yandex / VK / Sber / MTS Cloud** — high KYC + fast RKN
  compliance, structurally hostile to minimal-KYC relay use
- **RuWeb** — ToS explicitly bans anonymous-proxy / shell / Tor
- **DDoS-Guard-heavy reseller paths** — bad camouflage neighborhood
- **Generic no-KYC reseller storefronts** — ASN / jurisdiction
  unverifiable without post-allocation RDAP

## Mandatory pre-purchase verification (regardless of pick)

Each candidate must clear this checklist *before* committing budget.
The lead's failures came from skipping these; the reviewers'
critiques are downstream of them.

1. **RDAP-of-the-test-IP**: provision the cheapest available plan,
   run `curl -s "https://rdap.db.ripe.net/ip/<allocated-IP>" | jq .`,
   confirm the announced ASN matches the provider's claimed AS *and*
   is on the candidate ASN-disjoint-from-Selectel list.
2. **Official payment-page read**, not third-party blog summary or
   LLM-shortlist claim. Look for explicit BTC / USDT / SBP options
   on the vendor's own checkout flow, not a sub-page marketing
   "we accept crypto" through an aggregator with hidden surcharge.
3. **ToS read**: search vendor ToS for "anonymous", "proxy", "VPN",
   "Tor", "shell hosting". Some providers (e.g., RuWeb) explicitly
   ban; using them is a self-inflicted operational risk.
4. **DC building check**: cross-reference the allocated facility
   against datacentermap.com / vendor docs. Avoid stacking two
   candidates inside the same DataPro Moscow building (HOSTKEY +
   VDSina + ProfitServer co-locate there).
5. **Provider-name-to-AS sanity check**: at least two independent
   sources (bgp.tools / bgp.he.net + PeeringDB + ipip.net) should
   agree on the provider that announces the allocated /24. If the
   sources disagree, you are looking at a brand operating on
   borrowed infrastructure; treat the fate-isolation calculation as
   the *underlying* AS's, not the brand's.

## Methodology lessons

For future single-researcher → multi-reviewer audits:

- **A self-contained internal contradiction is the cheapest signal**
  that a researcher's shortlist needs review. The lead listed
  AS197695 as RUVDS's AS and as REG.RU's AS in the same document.
  This kind of error is mechanical, easy to miss, and worth running
  a re-read pass against before commitment.
- **Adversarial reviewers find divergent things.** Codex was best at
  vendor-page fact-checking (payment, ToS); Opus was best at L1
  fate-sharing and corporate-structure pattern matching (FirstByte
  → Media Land); Gemini was best at proposing geographically
  unconventional alternatives (Novosibirsk PoPs). The convergence
  came from the union of three distinct attack patterns. A single
  reviewer would have missed at least one axis.
- **Reviewers disagree about ASN attributions too.** Gemini and Opus
  both proposed JustHost but disagreed on its ASN (AS51659 vs
  AS207651). When two reviewers disagree, the only resolution is
  RDAP-of-the-allocated-IP at provisioning time — opinions about
  ASNs are not load-bearing without ground truth.
- **"Lead's recommendation does not survive review"** is not a
  failure of the lead — it is the expected outcome of a working
  audit. The multi-reviewer pass exists because single-agent
  research cannot self-audit for internal contradictions, vendor-
  marketing overclaims, or pattern-matched sanctioned-host
  structures. Expect to revise.

## Validation pass (2026-05-22)

The post-review re-ranking above (DataCheap, AdminVPS, HOSTKEY,
JustHost, RUVDS-conditional) was then run through a focused
pre-purchase verification pass against checks 2-5 of the checklist
(payment-page read, ToS scan, DC building cross-reference, two-source
ASN agreement). Check 1 (RDAP of the *allocated* IP) was deferred to
provisioning. A consumer-vantage rkn-check probe from an operator RU
Mac was also run (VPN-off) against the surviving candidates' marketing
domains to test for AS-level TSPU pre-blocking.

The validation surfaced enough new errors in the multi-review output
that the rank order shifted again.

### Resolutions of open questions

- **AS207651 = VDSina, not JustHost.** RIPE RDAP: AS207651 is
  `HOSTING-TECH` with registrant `vdsina-mnt`. The Opus-vs-Gemini
  disagreement over JustHost's ASN is resolved: **Gemini was right
  on AS51659** (`ASBAXET / LLC Baxet`); Opus's AS207651 was actually
  VDSina's, which is on the L1-overlap exclusion list at DataPro.
- **AS197695 = REG.RU's own AS, confirmed.** No RUVDS-branded
  prefixes appear on AS197695's BGP announcements. RUVDS has **no
  separately announced AS** — searching bgp.tools for "ruvds"
  returns zero results. RUVDS is structurally a brand operating on
  top of REG.RU's infrastructure, fate-shared with an explicitly-
  excluded provider.
- **AS59729 ≠ AdminVPS.** AS59729 is "ITL-BG" (Bulgarian, `GRFL-MNT`),
  unrelated. AdminVPS announces only from AS211183.

### New disqualifiers the multi-review missed

- **AdminVPS (AS211183) transits via REG.RU (AS197695).** bgp.he.net
  shows AdminVPS's upstreams are JSC Mastertel (AS29226) **and
  REG.RU (AS197695)**. Same fate-sharing defect that disqualifies
  RUVDS, but applied at the transit-upstream level rather than the
  announce-from-AS level. The multi-review did not flag this.
- **HOSTKEY ToS explicitly forbids the workload.** Their published
  terms-of-service prohibit (a) public proxy / VPN-as-a-service,
  (b) **VPN servers without prior approval** — including personal
  use, (c) Tor outbound. The BitPay-crypto-payment plus surfaced by
  the multi-review is real but does not help when the operational
  workload is itself ToS-forbidden.
- **RUVDS ToS adds an explicit anti-circumvention clause** on top
  of the AS197695 issue: prohibits "means to obtain access to
  resources with restricted access in the Russian Federation" plus
  private VPN/proxy with >10 GB/day total traffic.

### Corrections to multi-review claims

- **DataCheap is Moscow-only.** The "Kazan + Novosibirsk PoPs" claim
  in the multi-review is not corroborated by DataCheap's own About
  page; their single DC is a former-Yandex Tier-III facility at
  ul. Ugreshskaya in south Moscow. AS16262 announces multi-city
  prefixes but the physical operation is single-site.
- **JustHost's Khabarovsk PoP is not corroborated.** Novosibirsk
  and Kazan PoPs are confirmed; Khabarovsk was a multi-review
  hypothesis that did not survive verification against JustHost's
  own pages.

### Consumer-vantage rkn-check probe (2026-05-22)

Run from an operator RU consumer Mac with the chain off; the
bundled rkn-block-checker sweep returned `Likely in an RKN-blocked
zone (medium confidence)` (whitelist 20/21, blacklist 0/15 open
with 12 TLS-DPI silent drops + 2 timeouts) — confirming the vantage
is genuinely RU-consumer at probe time. The two surviving candidates'
marketing domains then probed cleanly:

| AS | Provider | Probe target | Verdict | TCP | TLS | PLT | DPI flags |
|---|---|---|---|---|---|---|---|
| AS16262 | DataCheap | datacheap.ru | ✓ OK | 37ms | 85ms | 170ms | none |
| AS51659 | Baxet (JustHost) | justhost.ru | ✓ OK | 59ms | 126ms | 321ms | none |

Caveat per [[s-tool-rkn-block-checker]]: rkn-check connects to the
domain's real IP, not to a relay-IP-with-SNI-override. This is the
*necessary* AS-reachability layer of the pre-purchase checklist
(neither AS is on TSPU's elevated-CIDR set), not the *sufficient*
end-to-end ASN-match-SNI validation — which happens at provisioning
with `openssl s_client -connect <allocated-IP>:443 -servername
<chosen-SNI>` per [[asn-match-sni-camouflage#validation-tool]].

### Final post-validation go/no-go

| Candidate | Pre-review | Post-multi-review | Post-validation | Reason for final state |
|---|---|---|---|---|
| RUVDS | #1 | conditional | **DO NOT BUY** | AS197695 = REG.RU; no own AS; ToS bans circumvention + >10GB/day VPN |
| AdminVPS | (not listed) | #2 | **DO NOT BUY** | REG.RU upstream transit (AS197695); not flagged by multi-review |
| HOSTKEY | (not listed) | #3 | **DO NOT BUY for this workload** | ToS bans Tor outbound + VPN servers without consent |
| JustHost (Baxet) | (not listed) | #4 disputed-ASN | **GO with caveats** (Novosibirsk PoP) | ASN confirmed AS51659; consumer-vantage clean; payment friction (no crypto, no SBP); ToS not yet read |
| DataCheap | (not listed) | #1 | **GO** | Clean own-AS, own-DC, independent upstreams, no anti-VPN ToS, consumer-vantage clean. Moscow-only; SBP-only payment |

### Final buy order (validated)

1. **DataCheap (AS16262, Moscow)** — buy first; the only candidate with
   no structural defects.
2. **JustHost / Baxet (AS51659, Novosibirsk PoP)** — buy second for
   genuine geographic orthogonality; ToS scan at signup is the
   remaining gate.
3. **Stop.** The other shortlist members are out for structural
   reasons. If a third candidate is needed, re-open the shortlist to
   providers outside this set (own AS, not on REG.RU transit,
   ideally crypto or SBP at checkout, no anti-VPN ToS).

### Methodology lesson added

A single round of adversarial multi-review is necessary but not
sufficient. The validation pass found errors the multi-review did
not catch — specifically, **upstream-transit fate-sharing is a
separate axis from the announce-from AS**: AdminVPS announces from
its own AS but transits via REG.RU, which the multi-review's
"two-source ASN agreement" check would not have caught because
both PeeringDB and bgp.tools agree on the announce-from AS. Add
an "upstream transit chain inspection" sub-step to the procurement
verification meta-discipline: pull the upstreams from bgp.he.net's
AS page and check the transit chain for excluded providers, not
just the announce-from AS.

This refinement is captured as step 6 (upstream-transit chain
inspection) of
[[hosting-provider-as-dpi-variable#procurement-verification]].

## What this synthesis does NOT settle

- **Whether RUVDS ever stands up a separately announced AS.** As of
  the 2026-05-22 validation pass it has none (AS197695 is REG.RU's),
  so it stays disqualified for fate-sharing. If that changes — a
  distinct AS for its KVM line — the fate-isolation calculation would
  need re-running and RUVDS could move back up the list.
- **Whether JustHost's far-east PoPs are sufficiently distinct from
  the Moscow / SPb IX cluster** to be worth the operational
  friction (smaller provider, less mature tooling, ASN ambiguity).
  Operator must weigh against the threat-model improvement.
- **Whether the pending Apr 2026 hosting-as-controller RU
  legislation** ([[s-2026-05-tspu-asn-camouflage-research]]) will
  collapse axis 2 of [[hosting-provider-as-dpi-variable]] before
  this shortlist matters. If it passes, all RU providers comply
  quickly and the "low-profile / niche" provider story breaks.

## Sources

- [[s-2026-05-multi-reviewer-vds-shortlist]]
- [[s-2026-05-ru-vps-ipv6-procurement-scan]]
- [[s-2026-05-tspu-asn-camouflage-research]]
- [[s-2026-05-xray-relay-community-reports]]
- [[hosting-provider-as-dpi-variable]]
