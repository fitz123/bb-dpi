# Source: `docs/architecture-decisions.md`

The in-repo consolidated architecture decisions register for the
`bb-dpi` project. 92 numbered decisions + 9 residual-risk
acknowledgements + 6 open TODOs, organized by domain.

- **Type**: in-repo synthesis doc
- **Location**: `/docs/architecture-decisions.md`
- **Ingested**: 2026-05-14
- **Format**: Decision / Why / Trade-offs / Citations per entry

## Key takeaways for the DPI-research domain

### Protocol layer
- [[reality-protocol]] is the load-bearing camouflage layer; everything
  else is a layer on top.
- Dual transport (XHTTP/443 + TCP+vision/8443) gives transport-level
  redundancy. See [[xhttp-transport]].
- `xPaddingBytes: "100-1000"` and `mode: auto` are the XHTTP-side
  defaults.
- `xver: 0` is non-negotiable on REALITY inbounds (never leak client
  IPs to third-party `dest`).

### SNI strategy
- [[asn-match-sni-camouflage]] is the per-server SNI selection
  heuristic.
- Strict ASN-match isn't always the right answer — empirical evidence
  beats the heuristic when it diverges. The doc records one case where
  retrying the provider-family SNI caused unpredictable wedges and
  was rolled back.
- Literal SNI only, never wildcard.

### Operational hardening
- `tcp_fast_open: false` (PR #11) for TCP-layer fingerprint coherence
  with real macOS apps.
- `sniffing: false` on the relay template (PR #11) to cut session
  metadata leakage on the more-exposed host.
- SSH password-class auth disabled via high-priority drop-in (PR #11);
  validates with `sshd -t` before reload.

### Workflow rules
- Branch → test on test client → PR → merge. Never push to main.
- Always run `/devops:dual-review` before `gh pr create` (PR #13 was
  the first to follow this rigorously).
- Always wait for Copilot + CI before declaring a PR ready.
- Never run `vpn-stop` from agent session (drops Claude Code's own
  connectivity).

### Diagnostic standing rules
- [[two-sided-tcpdump-diagnostic]] for dead-tunnel patterns.
- Mac-reboot-first when embedded `tsnet` loses corporate connectivity
  (config-debugging burns hours; reboot fixes in minutes).
- `tsnet "NoState. Ignoring authkey"` log is benign startup noise —
  don't recommend wiping state.

## Residual risks acknowledged

- [[latency-delta-active-probe]] — protocol-layer REALITY limit.
- [[utls-fingerprint-staleness]] — JA4 drift from real Chrome.
- No REALITY private key / `xhttp_path` / `short_id` rotation
  lifecycle.
- urltest cadence (30s + `interrupt_exist_connections: true`) is the
  documented trigger for [[dpi-flow-learning]].
- Single point of failure on the chain relay (accepted).

## Touched concept pages

- [[reality-protocol]] — §1, §2.11, §4
- [[asn-match-sni-camouflage]] — §4.1, §4.2, §4.3
- [[dpi-flow-learning]] — §7.5
- [[xhttp-transport]] — §1.2, §1.3, §1.4, §5.1, §5.2
- [[utls-fingerprint-staleness]] — §5.16, §7.3
- [[two-sided-tcpdump-diagnostic]] — §6.23
