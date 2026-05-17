---
tags: [tool, diagnostic, tspu, dns-poison, sni-dpi, http-stub, ipv6]
sources: [s-memory-twosided-tcpdump, s-2026-05-tspu-asn-camouflage-research, s-2026-05-ipv6-bgp-path-aws-stockholm]
updated: 2026-05-17
---

# Source: rkn-block-checker — RU consumer-vantage DPI diagnostic tool

A small Python CLI (MIT, on PyPI) that probes a RU-consumer-internet
vantage layer-by-layer and classifies failures by *type* of block: DNS
poisoning, TCP reset, TLS-DPI on SNI, or ISP HTTP stub-page. Useful as
a first-triage diagnostic complementary to
[[two-sided-tcpdump-diagnostic]].

- **Type**: external community tool (not a research compilation)
- **Repo**: [github.com/MayersScott/rkn-block-checker](https://github.com/MayersScott/rkn-block-checker)
- **PyPI**: `rkn-block-checker` (v0.5.0 at ingest)
- **License**: MIT
- **Ingested**: 2026-05-17

## What it does

Probes a built-in whitelist (sites that should always work in RU —
Gosuslugi, Yandex, Sberbank, VK, etc.) and blacklist (RKN-restricted —
Instagram, Twitter, Tor Project, ProtonVPN, etc.). For each target, runs
the stack independently:

```
DNS (system resolver + Cloudflare DoH compare)
  → TCP (raw connect, latency)
    → TLS (handshake to completion or classify reset/silent-drop)
      → HTTP (response + stub-page marker detection)
```

Each failure is classified into a documented signal pattern:

| Signal | What it means |
|---|---|
| `~ LIKELY TLS DPI` (reset after ClientHello) | TLS handshake reset right after ClientHello → consistent with SNI-based DPI |
| `~ LIKELY TLS DPI` (silent drop) | TLS handshake silently dropped → consistent with DPI filtering on ClientHello, but flag for path flakiness |
| `✗ DNS` | System DNS fails or returns disjoint addresses vs DoH → consistent with DNS poisoning |
| `✗ HTTP STUB` | HTTP body matches a known ISP stub-page marker (RU-language phrases narrowed to avoid false positives) |
| `DNS mismatch (...)` | Set-based DNS comparison — flagged only when system vs DoH address sets are completely disjoint, not partial overlap |

The set-based DNS comparison is worth highlighting:
[issue #5](https://github.com/MayersScott/rkn-block-checker/issues/5) led
to the fix — it ONLY flags rewriting when the address sets don't
intersect at all, which avoids false positives on multi-A-record CDN
sites.

## Why it's useful for this project

1. **First-triage diagnostic** before reaching for
   [[two-sided-tcpdump-diagnostic]]. When a site stops opening on a
   client vantage, `rkn-check --url <site>` localises the failure to
   DNS / TCP / TLS / HTTP layer in ~10 seconds without per-host setup.
   tcpdump is the right tool for "the tunnel is up but data doesn't
   flow" diagnosis; rkn-check is the right tool for "is this RKN/TSPU
   doing something to my browser or is my config wrong?"

2. **Longitudinal baseline** per vantage. Running `rkn-check --json`
   from cron and storing snapshots produces a time series of
   whitelist/blacklist verdicts per ISP/ASN — visible degradation
   across TSPU upgrades, without needing first-hand incident.

3. **SNI-camouflage validation**. Cross-checking a *deployed* camouflage
   SNI (`--url https://camouflage-host.tld`) against a *known burnt*
   SNI from history confirms the ASN-match doctrine
   ([[asn-match-sni-camouflage]]) is materially working on the
   client's reachability path, not just at the ASN-resolution level.

4. **Privacy hygiene by default**: `--no-self-info` flag suppresses the
   IP/ASN/location header; default User-Agent is generic Chrome
   ([issue #2](https://github.com/MayersScott/rkn-block-checker/issues/2)
   — `--identify` is an explicit opt-in if you want to mark traffic).

## Nuances to existing wiki claims

### IPv6 differential treatment

The tool's README explicitly states:

> "IPv4 only. Some Russian ISPs treat IPv6 differently (often less
> filtered) but the v4 path is what users actually experience in
> practice."

This is a **second operator-claim data point** (independent of the
deepwiki/rcd27 source) on the v4-vs-v6 question. It nuances the
deepwiki claim that "TSPU achieved IPv6 inspection parity Mar 2026"
captured in [[s-2026-05-ipv6-bgp-path-aws-stockholm#evidence-grade]].
The rkn-block-checker author, as of May 2026, takes the position that
"v6 often less filtered" remains operationally true at the consumer
level — even if TSPU has theoretically reached parity, deployment
heterogeneity across ISPs means consumer-vantage experience varies.

**Both claims remain low-confidence (single-source each), but they
point in opposite directions.** Captured here so the wiki preserves
the disagreement rather than picking a winner; the procurement
conclusion ("don't buy v6 for DPI evasion") stands either way because
the *evidence absence* leg from [[s-2026-05-xray-relay-community-reports]]
is the load-bearing argument, not the v6-parity claim.

### Signal patterns

The tool's documented classifiers correspond closely to wiki concepts:

- "TLS reset right after ClientHello — consistent with SNI-based DPI"
  → matches the SNI/IP correspondence drop documented in
  [[asn-match-sni-camouflage]] and the operator's first-party drop
  incident in [[s-memory-sni-asn-correlation-incident]].
- "TLS handshake silently dropped" → matches the time-windowed
  flow-learning + 15-20KB-freeze-on-foreign-IP patterns in
  [[dpi-flow-learning]] and
  [[s-2026-05-tspu-asn-camouflage-research]].
- "DNS mismatch (sys vs DoH)" → matches the cross-flow DNS-vs-TLS
  correlation surface that DoH for split-routed DNS mitigates.

The tool is **measurement-grounded**: classifiers are tied to
observable wire signals (RST timing, address-set disjoint), not
operator claim. This sits higher on the evidence ladder than
deepwiki/rcd27 (LLM-generated docs) or Habr QnA forum threads.

## Limitations

Per author's own README:
- IPv4-only probing
- Built-in target lists are small (~20 per category) — useful for a
  verdict, won't catch single-resource-specific blocks. Custom lists
  via `--white-file` / `--black-file`.
- One-shot snapshot, no retries, no longitudinal tracking built in
  (cron-friendly via `--json`).
- Stub-page markers are RU-language phrases — narrow enough to avoid
  matching news articles mentioning Roskomnadzor as content. New
  patterns added on report.

## Operational integration

For the chain-relay fleet, plausible cron job:

```cron
# rkn-check daily snapshot from RU consumer vantage
0 6 * * * ~/.local/bin/rkn-check --no-self-info --json >> ~/rkn-monitor/snapshots.jsonl
```

After deploying a new camouflage SNI via `make deploy`, prompt
verification:

```sh
rkn-check --url https://<new-sni>.tld
```

If the verdict is anything other than `✓ OK`, the camouflage host
itself is being filtered — abort the rollout before clients see it.

## Touched concept pages

- [[two-sided-tcpdump-diagnostic]] — rkn-check is the lighter-weight
  first-triage tool; tcpdump is the heavier "tunnel up, payload
  dropped" diagnostic. Complementary, not redundant.
- [[asn-match-sni-camouflage]] — tool's SNI-DPI classifier is the
  practical validation surface for the ASN-match strategy.
- [[dpi-flow-learning]] — tool's "silent drop" classifier maps to the
  flow-learning signal class.
- [[dns-aaaa-cascade-failure]] — first-party investigation of a tool
  `✗ DNS` verdict revealed the AAAA-cascade mechanism; tool's
  classifier correctly catches it as a DNS-layer failure but the
  underlying mechanism is more specific than classical DNS poisoning.

## Sources

- [github.com/MayersScott/rkn-block-checker](https://github.com/MayersScott/rkn-block-checker)
- [PyPI: rkn-block-checker](https://pypi.org/project/rkn-block-checker/)
- README and acknowledged contributor issues
  ([#1](https://github.com/MayersScott/rkn-block-checker/pull/1),
  [#2](https://github.com/MayersScott/rkn-block-checker/issues/2),
  [#3](https://github.com/MayersScott/rkn-block-checker/pull/3),
  [#4](https://github.com/MayersScott/rkn-block-checker/issues/4),
  [#5](https://github.com/MayersScott/rkn-block-checker/issues/5))
