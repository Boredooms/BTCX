# BCTX Phase 8 — Offline GeoIP / ASN Runtime

The GeoIP runtime annotates network observations with a country and ASN for the
Geo Map screen and the `bctx geo inspect` CLI. It is a **strictly-offline leaf**
(`tui/geoip`): it dials nothing, downloads nothing, and reaches no acquisition
or monitoring code. BCTX never fetches a database — the investigator supplies a
free DB-IP `.mmdb` and installs it locally; every lookup reads that installed
file read-only.

Grounded in `tui/geoip/{geoip,registry,errors}.go` and `cli/commands/geo.go`.
The no-network property is enforced by
`tests/offline/boundary_test.go:TestTUIPackagesHaveNoHTTPTransport` (the
strictly-offline leaf branch).

---

## 1. Install location and registry

Databases install under `~/.bctx/geoip/` (resolved by `geoip.Dir()`). A local
metadata registry at `~/.bctx/geoip/registry.json` records provenance and an
integrity hash — **and no secrets**:

```
{ db_name, db_type ("country"|"asn"), source, source_url, version, license,
  downloaded_at, installed_at, sha256, record_count, path }
```

Entries are keyed by `db_type`, so a country database and an ASN database are
tracked independently.

---

## 2. Lifecycle (`bctx geo …`, fully offline)

| Verb | Behavior |
|------|----------|
| `geo status` | Prints each installed database's provenance + sha256 + path, or the honest `GEOIP DATABASE NOT INSTALLED — install with: bctx geo install <file>` with the install path. |
| `geo install <file.mmdb>` | Copies the file into `~/.bctx/geoip/`, computes its sha256 and record count via the MMDB reader, auto-detects/records `db_type`, and writes the registry atomically (temp file + rename). Provenance flags (`--name/--type/--source/--source-url/--version/--license`) default to the DB-IP IP-to-Country Lite metadata. |
| `geo verify <file.mmdb>` | Recomputes the file's sha256 and compares it to the registry: `VERIFY OK`, `VERIFY FAILED` on mismatch, or the not-installed message. |
| `geo inspect <ip>` | Opens the installed databases read-only and resolves country + ASN for one IP; a missing record prints `(no record)`, a missing database prints the install message — never a fabricated lookup. |

---

## 3. Reader

`geoip.Open(dir)` opens the installed `.mmdb` read-only via
`github.com/oschwald/maxminddb-golang` (accepted by the Phase 8 dependency gate:
pure-Go, builds under `CGO_ENABLED=0`, ISC license, no transport imports — only
`net`/`net/netip` IP value types). A `netip.Addr` is converted to the reader's
`net.IP` by a pure value conversion (`net.IP(ip.AsSlice())`) — no name
resolution, no socket. Country and ASN are separate MMDB files keyed in the
registry by `db_type`.

The `DB` interface is `LookupCountry(netip.Addr)`, `LookupASN(netip.Addr)`,
`Status()`, `Close()`.

---

## 4. Honest degrade (AGENTS §16/§19)

- No database installed → `ErrNotInstalled`, surfaced as the documented install
  message. The Geo Map screen shows the honest no-DB state and does not plot
  located points.
- sha256 mismatch on verify → `VERIFY FAILED`; the file is treated as untrusted,
  never silently used.
- No record for an IP → `(no record)`, never a guessed country or ASN.

Country/ASN values are presented as **observation metadata**, never as proof of
ownership or location of a person (design §9; AGENTS §18).
