# XHTTP transport

XHTTP is xray-core's HTTP/2-like transport for VLESS+REALITY. It frames
the tunnel as a sequence of HTTP request/response exchanges, exposing
a *web-traffic-shaped* signature on the wire instead of the
single-long-TLS-stream shape of TCP+vision.

XHTTP is the project's primary RU-egress transport, with TCP+vision
retained as a fallback.

## Wire shape

Real Chrome → CDN traffic for a busy site is bursty: many short
HTTP/2 requests, variable-size, with random idle gaps. XHTTP imitates
that by chunking the VLESS payload into HTTP POST/GET exchanges over
a long-lived HTTP/2 connection (under REALITY-wrapped TLS).

Compared to the alternative TCP+vision:

| Property | XHTTP/443 | TCP+vision/8443 |
|---|---|---|
| Looks like | HTTP/2 to a CDN | Long-lived single TLS stream |
| Per-request boundaries | Yes (variable) | None visible |
| Padding | `xPaddingBytes` random per request | None (single stream) |
| DPI signature | Web-like, varied | Distinct large-encrypted-stream |
| Empirically resilient on RU consumer DPI 2026 | Yes (primary) | No (selectively burnt) |

## Config knobs that matter

Server inbound:
```jsonc
"streamSettings": {
  "network": "xhttp",
  "xhttpSettings": {
    "path": "/<server-specific-random-hex>",
    "mode": "auto",
    "xPaddingBytes": "100-1000"
  },
  "security": "reality",
  "realitySettings": { /* see [[reality-protocol]] */ }
}
```

- **`mode: "auto"`** — xray chooses `packet-up` (POST-per-chunk, more
  HTTP-like, lower throughput) vs `stream-up` (single-POST streaming,
  higher throughput, more HTTP/2-like) per-condition. The auto picker
  tilts toward the more HTTP-shaped variant when middleboxes might be
  inspecting chunk boundaries.
- **`xPaddingBytes: "100-1000"`** — random padding bytes appended per
  request. Defeats fixed-size-distribution statistical attacks on
  REALITY-tunneled HTTP. Adds 5-15% bandwidth overhead, accepted.
- **`path`** — per-server random hex string (8 bytes / 16 chars in
  the project's deploy script). Looks like an arbitrary asset path
  inside the encrypted tunnel. Encrypted under REALITY TLS but
  visible if the REALITY key ever leaks, and visible to anyone with
  the client's bundle.
- **No `host` field** — leave the HTTP `Host` header default. Setting
  it explicitly can break REALITY's masquerade. Documented empirical
  regression in [[s-memory-chain-relay-rationale]].

## XHTTP vs TCP+vision: why XHTTP wins on RU egress

The 2026-05-13 incident that drove the project's preference (recorded
in [[s-memory-sni-asn-correlation-incident]]):

- A TCP+vision outbound to a cloud-region exit was burnt by RU
  consumer DPI: PSH packets carrying REALITY+vision payload were
  silently dropped in-path. TCP handshake completed; data didn't flow.
- The SAME server's XHTTP/443 inbound, also REALITY, with same SNI
  family — *was unaffected*. Different framing, different DPI
  classifier, different outcome.
- Diagnosis required [[two-sided-tcpdump-diagnostic]] because the
  drop was payload-aware, not connection-aware.

The takeaway encoded in [[s-arch-decisions]] §C5: for RU paths,
prefer `--proto xhttp` (renders only `xhttp-*` outbounds in urltest);
the TCP+vision inbound stays available on the server but isn't
actively probed by clients.

## Client-side: the two-process split

XHTTP is implemented in xray-core, not sing-box. The project's client
runs both:

- **xray-core**: provides per-server SOCKS proxy on `127.0.0.1:1080+i`
  per visible server. Each SOCKS port is a SOCKS-to-VLESS-XHTTP-REALITY
  bridge.
- **sing-box**: owns the TUN, DNS, urltest, and routing. Routes
  user-app traffic into one of the SOCKS ports based on urltest's
  current selection.

For each visible server in `servers.json`, render-config produces a
sing-box `xhttp-<name>` outbound (type=socks, server_port=1080+i)
matched by xray-core's `socks-in-<name>` inbound on the same port.
Filter must run before `to_entries[]` to keep indices sequential
across the visible subset.

See [[s-arch-decisions]] §5.1, §5.2.

## What XHTTP does NOT solve

- **uTLS staleness** — same risk as any REALITY transport. See
  [[utls-fingerprint-staleness]].
- **TCP-layer fingerprint** — same risk. TFO=true was historically a
  divergence flag; flipped to false in PR #11 specifically to keep
  the TCP-layer signature coherent with what real macOS apps emit.
- **Latency-delta active probe** — REALITY-level, not XHTTP-specific.
  See [[latency-delta-active-probe]].

## Sources

- [[s-arch-decisions]] §1.2, §1.3, §1.4, §5.1, §5.2, §C5
- [[s-memory-sni-asn-correlation-incident]] (the burn incident that motivated the
  preference)
- [[s-memory-chain-relay-rationale]] (Host-field gotcha)
