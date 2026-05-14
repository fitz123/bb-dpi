# Source: project memory — chain-relay rationale

Out-of-repo memory file documenting the introduction of a VLESS chain
relay on a same-region datacenter, used to restore cloud-region exit
access after consumer ISP DPI rendered the direct path unreliable.

- **Type**: out-of-repo project memory
- **Ingested**: 2026-05-14
- **Subject**: VLESS chain topology, ASN-match SNI strategy, deploy
  gotchas

## Topology

```
client (consumer ISP)
  → REALITY+XHTTP → relay (in-region datacenter)
  → REALITY+XHTTP → cloud-region exit
  → freedom → internet
```

Two REALITY hops. The first hop's wire shape is consumer-ISP →
in-region datacenter, with same-AS SNI camouflage; the second is
datacenter → cloud, on a different ASN profile than the
problematic direct consumer-ISP → cloud path.

## Why two hops

The direct consumer ISP → cloud exit was DPI-burnt by the
(SNI, ASN) correlation drop documented in
[[s-memory-sni-asn-correlation-incident]]. Adding a same-region datacenter relay
moves the cross-border leg from "consumer-ISP → cloud" to
"datacenter-AS → cloud", which is not on the DPI hot-list.

The trade-off: the relay is a single point of failure for the cloud
exit. If the relay goes down, the chain-served upstream is unreachable.
The project's other exit (a non-cloud direct exit on a different AS)
is independent and survives a relay outage — only chain-dependent
traffic is affected.

## ASN-match SNI on the relay

The relay's REALITY `dest`/`serverNames` is chosen on the SAME AS
as the relay's host IP. Specifically: a CDN-CNAMED consumer-site
hostname that resolves into the same provider's CDN pool. This makes
the first-hop wire pattern look like "consumer client visiting a
consumer site on the same provider" — the most-plausible flow shape
available for in-region camouflage.

The candidate dest had to pass the WAF check: some CDN frontends
reject vanilla TLS ClientHellos (visible WAF interference makes them
unusable as a REALITY `dest`). The chosen hostname is on an
object-storage-style edge that accepts unmodified handshakes.

See [[asn-match-sni-camouflage]] for the general strategy.

## Implementation notes

- The relay runs the same xray image as exits, with a different
  template (`config/server-relay.template.json`). Two REALITY
  inbounds + two `to-upstream-*` REALITY outbounds chaining to the
  upstream's host/keys/SNI. No `freedom` outbound — pure relay,
  fail-closed.
- The relay's outbound auths to the upstream as a synced user
  (whose name matches the relay's name, by convention). That user
  must exist on the upstream's `clients` list — `xray-users`
  propagates UUIDs to all servers regardless of `client_render`.
- Routing rules statically pin `xhttp-in → to-upstream-xhttp` and
  `tcp-in → to-upstream-tcp`. No sniffing-driven routing on the
  relay (sniffing is disabled).

## Deploy gotchas captured

The memory file specifically calls out these:

- **`docker compose up -d` doesn't fully refresh xray's resolver
  state** for the new `dest` hostname after a redeploy. Observed:
  swapping the relay's SNI returned the OLD dest's cert on probes
  for several minutes until an explicit `docker compose restart xray`
  cleared the in-process state. PR #11 added this restart to
  `start_container()`.
- **`generate_keys()` regex** had to be hardened to match both old
  (`Public key: ...`) and new (`Password (PublicKey): ...`) xray
  output formats. Trivial-looking patch, real prod-breaking issue
  when xray-core's CLI shape changed in an `:latest` update.
- **`xray-users add` silent-fail under `set -euo pipefail`**: the
  script exits early if any inner command (remote ssh+jq+restart)
  returns non-zero, leaving the user UUID partially propagated.
  Workaround: manual ssh+jq+restart on the skipped servers. TODO
  filed: wrap loop body in `|| { failed+=("$host"); }`.

## Touched concept pages

- [[reality-protocol]] — operational notes on dest-change restart
- [[asn-match-sni-camouflage]] — relay's SNI choice
- [[xhttp-transport]] — Host-field gotcha
- [[dpi-flow-learning]] — motivation for the chain

## Caveats

- The relay model trades availability (single PoF) for camouflage.
  Acceptable for a small fleet where alternative exits exist for
  non-chain-dependent traffic.
- The same-AS-as-relay SNI strategy is only viable where a usable
  consumer-site CDN exists on the relay's provider. Not all hosting
  providers have plausible camouflage targets in their AS.
