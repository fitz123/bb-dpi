# uTLS fingerprint staleness

[[reality-protocol]] relies on uTLS (a Go library) to make the client's
TLS ClientHello look indistinguishable from a real Chrome/Safari/
Firefox ClientHello. The `fingerprint: "chrome"` knob picks a frozen
Chrome ClientHello shape that ships in the xray-core / sing-box
binary at build time.

**The gap**: real Chrome's actual ClientHello evolves every release
(~4-6 weeks). uTLS's "chrome" snapshot lags. If a censor builds a
JA3/JA4 list of real-Chrome ClientHello hashes and flags everything
that *looks like Chrome* but doesn't *match the current Chrome*, the
proxy stands out.

## Why the staleness is observable

A typical user's Mac runs:

- Auto-updating real Chrome → ClientHello matches the current Chrome
  release.
- The bb-dpi VPN client → ClientHello matches whatever Chrome version
  the brew-installed xray-core's uTLS was built against (possibly N
  releases behind current).

Both originate from the same source IP within seconds of each other,
to overlapping destination categories. A passive JA4 collector with
an IP-correlation pipeline partitions the Mac's traffic into:

- Cluster A: real-Chrome JA4 → connects to actual CDN ranges.
- Cluster B: uTLS-snapshot JA4 → connects to a VPS IP, not a CDN.

That partition is a near-zero-false-positive proxy discriminator on
top of everything else REALITY does.

This was flagged in the post-PR-#11 architecture audit and remains
an accepted residual risk because of mitigation cost — proper
rotation requires per-Chrome-release coordinated binary updates
across server + client + tracked-config, which isn't worth building
for the current threat model.

## What "chrome" actually means

`fingerprint: "chrome"` in xray-core / sing-box config resolves to
whatever Chrome version the bundled uTLS implementation was tested
against when the binary was built. Concrete values shift between
xray-core and sing-box releases:

- xray-core's `transport/internet/reality/reality.go` uses uTLS's
  `HelloChrome_Auto`, which uTLS itself updates as Chrome ships new
  shapes.
- sing-box's TLS layer uses uTLS via its `utls.fingerprint` field; the
  value `"chrome"` maps to `HelloChrome_Auto` in current sing-box.

Both follow uTLS upstream's release cadence (irregular). Drift
between Chrome stable and uTLS-bundled-chrome shows up as different
GREASE values, different post-quantum key share inclusion, different
extension ordering on TLS 1.3.

The practical drift window is 1-3 binary releases — typically a few
weeks to a few months.

## What's been observed in the wild

- RU DPI (TSPU and adjacent) has been documented running TLS
  fingerprint analysis since 2023 in academic and industry research.
- The specific "uTLS-stale JA4 with co-occurring real-Chrome JA4 from
  the same client IP" attack is plausible and cheap; whether it's
  *deployed at scale today* against the project's threat model is not
  confirmed in this project's memory.

The architecture audit explicitly distinguishes "theoretical capability"
from "observed in deployed RU DPI per project memory or public
research". uTLS staleness is in the former bucket today.

## Mitigations

### Active mitigations (none deployed yet)

- **Auto-update sing-box + xray-core regularly.** Both projects bump
  their uTLS dependency in each release. Auto-updates close most of
  the drift. Server side: `docker compose pull` on every deploy is
  already automatic (the image tag is `:latest`). Client side: macOS
  brew has no enforced auto-update cadence; the project doesn't bake
  one in. Manual `brew upgrade sing-box xray` is the workaround.
- **Per-server fingerprint variation.** `fingerprint` could be a
  per-server field in `servers.json` so different servers vary
  between `chrome`, `firefox`, `safari`. Breaks cross-fleet
  correlation but adds operational complexity. Not deployed.
- **Pin to a specific brew version.** Document a tested
  (sing-box version, xray version) tuple per quarter; pin install
  instructions to that tuple; bump in lockstep with Chrome stable
  observations. High maintenance.

### Defensive observation

If the staleness ever *does* manifest as a real burn, the diagnostic
signature is:

- Specific tunnel outbounds start failing for *new* connections (old
  long-lived flows survive — they were established before the
  classifier was triggered).
- TCP handshakes complete; TLS ClientHello succeeds; the
  server-side TLS handshake response is sometimes followed by a
  connection RST or PSH-drop. See [[two-sided-tcpdump-diagnostic]].
- The smoking gun would be: capture the ClientHello on the wire,
  compute its JA4 hash, compare to a sample of real-Chrome JA4 from
  the same source IP. Divergence = the classifier has signal.

The project hasn't run that comparison yet; it's a worthwhile
diagnostic to keep in pocket.

## Sources

- xray-core source: `transport/internet/reality/reality.go`
- sing-box docs: [TLS UTLS configuration](https://sing-box.sagernet.org/configuration/shared/tls/)
- Academic: published TLS-fingerprinting research is broad; specific
  RU-DPI deployment of JA3/JA4 not directly cited in this wiki yet.
