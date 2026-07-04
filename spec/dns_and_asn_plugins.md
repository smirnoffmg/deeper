# Spec: DNS-record and IP-intelligence plugins

Status: implemented (`internal/pkg/plugins/dns_records/`, `internal/pkg/plugins/ip_intel/`).

**Post-implementation fixes**, found via live testing against `codescoring.ru`
in an environment where raw UDP DNS-53 queries are refused but HTTPS/OS-native
resolution works fine (confirmed: `LookupIPAddr`/`LookupAddr` succeed,
`LookupMX`/`LookupTXT`/`LookupNS`/`LookupCNAME` always fail there):

1. `dns_records`'s `FollowTrace` used to abort entirely (returning an error,
   losing all results) whenever any stdlib lookup failed, *before* even
   attempting the independent DoH-backed SOA/CAA lookups — even though DoH
   worked fine and had real data (confirmed: `codescoring.ru`'s SOA record
   decodes to a genuinely new email, `support@selectel.ru`). Fixed: every
   lookup (MX/TXT/NS/CNAME/SOA/CAA) is now independent — one failing logs a
   warning and is skipped, never blocking the others.
2. A related, more serious bug surfaced in `internal/pkg/workerpool/
   workerpool.go`: `Worker.processTask`'s send to a task's `ReplyTo` channel
   was gated on the per-task context, which raced against the buffered send
   whenever a plugin call (e.g. `CrtShPlugin`) ran longer than `TaskTimeout` —
   plugins take no context, so a slow call can't be aborted early, and by the
   time it returned, the per-task ctx had already expired, so the `select`
   could nondeterministically drop an already-computed result instead of
   delivering it. This hung `Processor.ProcessTrace`'s collection loop
   waiting for a result that would never arrive, discarding an entire scan's
   results until the outer scan-wide timeout (5 minutes) fired. Fixed:
   `ReplyTo` sends are now unconditional (the buffer is always sized to
   guarantee no blocking by the caller's own contract).
3. `ip_intel`'s ASN lookup failures were completely silent (no log at all),
   making it indistinguishable from "this IP genuinely has no ASN data"
   (impossible — every routable IP has one). Fixed: lookup errors are now
   logged as warnings, distinct from the legitimate empty-response case.

## Context

Grepping every `TraceType` constant in `entities.go` against actual usage
across the codebase (not just its own declaration) shows **41 of 62 declared
trace types are pure dead vocabulary** — never produced, never consumed,
anywhere. Two clusters of those are cheap to close and directly useful for
recon: DNS record types beyond the A/AAAA `dns_resolver` already handles
(`DnsRecordMX`, `DnsRecordTXT`, `DnsRecordNS`, `DnsRecordCNAME`, plus
`DnsRecordSOA`/`DnsRecordCAA`, which need a different lookup mechanism — see
below), and `ASN`/`Netblock`, which close `entities.IpAddr` as a plugin
*input* — today it's a dead end exactly like `Email`/`Github` were before
the identity-enrichment spec.

Both plugins below need **zero new third-party dependencies**: everything
either goes through Go's stdlib `net.Resolver` (same as `dns_resolver`
already uses) or Google's public DNS-over-HTTPS JSON API via the existing
shared `deeperhttp.Client` (same pattern `contact_crawler` already uses for
plain HTTP).

## Plugin 1: `dns_records` — MX / TXT / NS / CNAME / SOA / CAA

Consumes `Domain` and `Subdomain` (same two input types `contact_crawler`
already registers for — this plugin runs alongside it, not instead of it).

**Stdlib-backed lookups** (verified via `go doc`, matching `dns_resolver`'s
existing style exactly — same `context.Context`-based `net.Resolver` calls,
just different methods):

```go
type dnsLookups interface {
    LookupMX(ctx context.Context, name string) ([]*net.MX, error)
    LookupTXT(ctx context.Context, name string) ([]string, error)
    LookupNS(ctx context.Context, name string) ([]*net.NS, error)
    LookupCNAME(ctx context.Context, host string) (string, error)
}
```

- MX → `entities.Trace{Type: entities.DnsRecordMX, Value: mx.Host}` per
  record (drop the numeric preference — not part of the vocabulary's value
  slot; if it matters later, revisit whether `Trace` needs a metadata field,
  don't add one speculatively now).
- TXT → `entities.Trace{Type: entities.DnsRecordTXT, Value: txt}` per
  record. Expect noise (SPF/DKIM/domain-verification strings) — that's
  fine, it's exactly the OSINT-relevant content ("this domain has a
  `google-site-verification=`/`facebook-domain-verification=` TXT record"
  is itself the finding, not something to filter out).
- NS → `entities.Trace{Type: entities.DnsRecordNS, Value: ns.Host}` per
  record — reveals the DNS hosting provider.
- CNAME → `entities.Trace{Type: entities.DnsRecordCNAME, Value: cname}`,
  **only when `cname != queried name`** (stdlib returns the name itself,
  dot-terminated, when there's no real CNAME — must not emit a
  self-referential no-op trace). This is the one most likely to reveal a
  third-party SaaS vendor directly (e.g. a subdomain CNAMEd to
  `shops.myshopify.com` or `*.zendesk.com`).

**SOA and CAA are not exposed by Go's stdlib `net.Resolver`** (confirmed via
`go doc net.Resolver` — only `LookupAddr/CNAME/Host/IP/IPAddr/MX/NS/
NetIP/Port/SRV/TXT` exist; no `LookupSOA`/`LookupCAA`). Rather than pull in
a full DNS library (e.g. `miekg/dns`) for two record types, use **Google's
public DoH JSON API** (verified against current docs: plain unauthenticated
`GET https://dns.google/resolve?name={domain}&type={SOA|CAA}`, no API key,
JSON `Answer[].data` field holds the record text) through the existing
`deeperhttp.Client` — zero new Go dependency, same HTTP-boundary-injection
pattern `contact_crawler` already uses.

```go
type dohFetcher interface {
    Get(ctx context.Context, url string) (*http.Response, error)
}
```

- CAA → `entities.Trace{Type: entities.DnsRecordCAA, Value: data}` per
  answer.
- SOA → `entities.Trace{Type: entities.DnsRecordSOA, Value: data}` for the
  raw record, **plus** a second, more interesting derived trace: parse the
  SOA's RNAME field (the second space-separated token in the `data` string,
  e.g. `ns1.example.com. hostmaster.example.com. 2024010100 ...`) — RNAME
  encodes an administrative contact email with the `@` replaced by `.`, per
  the DNS SOA RFC convention. `hostmaster.example.com.` → `hostmaster@
  example.com`. This needs care: **the first unescaped dot is the
  separator**; a mailbox local-part containing a literal dot is escaped as
  `\.` in the RNAME per the RFC, so a naive `strings.SplitN(rname, ".", 2)`
  is wrong for those cases — handle the escape, or at minimum detect and
  skip escaped-dot RNAMEs rather than emit a corrupted email (document
  whichever is chosen; don't silently emit wrong data). Emit
  `entities.Trace{Type: entities.Email, Value: rname-decoded}` — this is a
  genuinely new, previously-unconsidered email source, independent of
  anything `contact_crawler` or the identity-enrichment plugins find.

## Plugin 2: `ip_intel` — ASN / Netblock / PTR

Consumes `entities.IpAddr` — currently a pure dead end as a plugin input,
same gap `Email`/`Github` had before this session's other spec.

**ASN + Netblock**, via Team Cymru's IP-to-ASN DNS interface (verified
format via web search against Team Cymru's own documentation — this is a
plain DNS TXT lookup, no HTTP, no whois-protocol implementation, fits the
exact same `net.Resolver.LookupTXT` stdlib call `dns_records` above already
uses):

```go
type txtLookup interface {
    LookupTXT(ctx context.Context, name string) ([]string, error)
}
```

1. Reverse the IPv4 octets and query
   `{d}.{c}.{b}.{a}.origin.asn.cymru.com` (IPv6: nibble-reversed,
   `origin6.asn.cymru.com` — v1 can scope to IPv4 only and document IPv6 as
   a follow-up, since every IP `dns_resolver` has produced so far has been
   IPv4). Response format (pipe-delimited):
   `"ASN | BGP Prefix | Country Code | Registry | Allocation Date"`.
2. Parse the ASN number and BGP prefix out of field 1 and 2.
   `entities.Trace{Type: entities.ASN, Value: "AS" + asnNumber}`,
   `entities.Trace{Type: entities.Netblock, Value: bgpPrefix}`.
3. Optionally, a second chained lookup — `AS{asnNumber}.asn.cymru.com TXT`
   — returns `"ASN | Country | Registry | Allocated | AS Name"`, where AS
   Name is the human-readable org/hosting-provider name (e.g.
   `"HETZNER-AS, DE"`). Emit `entities.Trace{Type: entities.Company, Value:
   asName}` — a deliberate, judgment-call reuse of `Company` for "the
   organization that owns this IP's ASN," which is a legitimate reading of
   that trace type even though it's phrased for business entities elsewhere
   in the vocabulary; flag this choice in code comments so a future reader
   understands it's a deliberate reuse, not confusion with `contact_crawler`
   discovering an actual employer name.

**PTR (reverse DNS)**, stdlib, single call:

```go
type addrLookup interface {
    LookupAddr(ctx context.Context, addr string) ([]string, error)
}
```

`entities.Trace{Type: entities.DnsRecordPTR, Value: hostname}` per returned
name. Reverse hostnames often leak the hosting provider even without the
ASN lookup (e.g. `static.198-51-100-5.clients.your-server.de` names Hetzner
directly in the string) and occasionally leak internal naming conventions.

**Design note on scope**: ASN/Netblock lookup and PTR lookup are bundled
into one `ip_intel` plugin rather than split further, since both are "passive
facts derivable from a bare IP address" — the same granularity `dns_resolver`
already uses for "all A/AAAA records for a subdomain." `entities.Netblock`
vs. the already-dead `entities.IPRange` are near-duplicate vocabulary
entries; this plugin uses `Netblock` for the BGP-announced CIDR (the
standard RIR/BGP term for what Team Cymru returns) and leaves `IPRange`
alone — worth reconciling later, not in scope for this pass.

## Package layout

```
internal/pkg/plugins/dns_records/
  main.go          # plugin shell, NewPlugin, Register (Domain, Subdomain), FollowTrace, String
  main_test.go
  stdlib_records.go     # MX/TXT/NS/CNAME via net.Resolver
  stdlib_records_test.go
  doh_records.go         # SOA/CAA via Google DoH JSON, + SOA-RNAME email decoding
  doh_records_test.go

internal/pkg/plugins/ip_intel/
  main.go          # plugin shell, NewPlugin, Register (IpAddr), FollowTrace, String
  main_test.go
  asn.go           # Cymru DNS TXT chained lookups + parsing
  asn_test.go
  ptr.go           # LookupAddr wrapper
  ptr_test.go
```

Both follow the `dns_resolver` template exactly: pointer-receiver struct,
`NewPlugin()` wiring `net.DefaultResolver` (and, for `dns_records`,
`deeperhttp.NewClient(config.LoadConfig())` for the DoH calls), injectable
interfaces at every network boundary, fake struct literals in tests — no
live network calls in any test.

## Test plan (TDD order, per plugin)

`dns_records`:
1. `stdlib_records_test.go` via a fake `dnsLookups`: MX/TXT/NS map straight
   to traces; CNAME self-referential response (`cname == queried+"."`)
   produces no trace; CNAME to a different host produces one.
2. `doh_records_test.go` via a fake `dohFetcher` returning canned JSON:
   CAA/SOA raw traces; **SOA RNAME decoding** — table-driven: plain RNAME
   (`hostmaster.example.com.` → `hostmaster@example.com`), RNAME with an
   escaped dot in the local part (assert either correct decoding or a
   documented skip, not corrupted output), malformed/short SOA data (no
   panic, no trace).
3. `main_test.go`: wrong input type → `nil, nil`; `String()`; one domain
   with all four record types present, asserting the full trace set.

`ip_intel`:
1. `asn_test.go` via a fake `txtLookup`: known Cymru-format TXT response →
   correct ASN/Netblock/Company traces; malformed/empty TXT response → no
   traces, no panic; second chained AS-name lookup failing independently
   doesn't block the first lookup's ASN/Netblock traces (partial success is
   still useful).
2. `ptr_test.go` via a fake `addrLookup`: known hostname(s) → traces;
   `LookupAddr` error (common for IPs with no PTR configured) → `nil, nil`,
   not a plugin error (absence of PTR is the normal case, not a failure).
3. `main_test.go`: wrong input type → `nil, nil`; `String()`.
4. **Not automated**: a real call against `codescoring.ru`'s already-known
   IPs (`176.57.66.104`, etc.) and a real domain's MX/TXT/NS/SOA, same
   deferred-manual-smoke-test convention as the other plugin specs.

## Verification

1. `go build ./...`, `go vet ./...`, `golangci-lint run ./...` clean.
2. `go test -race ./...` green, including all new plugin test files.
3. `./build/deeper plugins list` shows `dns_records` under `domain` and
   `subdomain`, and `ip_intel` under `ip_addr` (a new row — `ip_addr` has no
   registered consumer today).
4. Manual smoke run against `codescoring.ru`: confirm MX/NS/TXT records
   come back for the domain and known subdomains; confirm ASN/Netblock
   resolve correctly for the IPs already in `trace_edges` from the earlier
   scan (cross-check against a public source like bgp.he.net for the same
   IPs, same independent-verification discipline used to catch the worker
   pool race earlier).

## Critical files referenced

- `internal/pkg/entities/entities.go` — `DnsRecordMX/TXT/NS/CNAME/SOA/CAA/
  PTR`, `ASN`, `Netblock`, `Email`, `Company` trace types already exist; no
  vocabulary additions needed.
- `internal/pkg/plugins/dns_resolver/main.go` — template to follow; confirms
  the `net.Resolver`-via-injectable-interface pattern already used for
  A/AAAA.
- `internal/pkg/plugins/contact_crawler/main.go` — confirms the
  `deeperhttp.Client`-via-injectable-interface pattern for the DoH calls.
- `internal/pkg/http/client.go` — shared HTTP client, reused for the DoH
  lookups (no new dependency).
- `internal/pkg/state/state.go` — confirms `IpAddr` has no registered
  consumer today; this spec adds one.
