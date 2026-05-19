#!/bin/bash
# Deterministic environment for golden generation.
# Sourced by generate.sh and any pkg/render test that needs to mirror
# the bash render-config's envsubst inputs.
#
# Values are intentionally fixed and obviously-synthetic so the goldens
# are reproducible byte-for-byte and PII-free.

export UUID="00000000-0000-0000-0000-000000000000"
export FLOW="xtls-rprx-vision"
export FINGERPRINT="chrome"
export TAILSCALE_AUTH_KEY="auth-key-deterministic-fake-test"
export TAILSCALE_HOSTNAME="test-host"
export INTERNAL_DNS_1="10.0.0.53"
export COMPANY_DOMAIN="example.invalid"
# Sourced only by generate.sh / parity tests in a subshell context.
# This mutates HOME in the caller; do NOT `source` from your interactive
# shell — run generate.sh as a child process instead.
export HOME="/Users/example"
