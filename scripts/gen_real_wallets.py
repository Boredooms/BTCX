#!/usr/bin/env python3
"""Generate three LOCAL, air-gapped demo cases keyed to REAL mainnet Bitcoin
addresses, with locally-generated-but-realistic transaction histories that
produce a clean LOW / MEDIUM / HIGH risk ladder.

The ADDRESSES are real, well-known mainnet addresses. The TRANSACTION history is
synthetic (we have no network here), shaped so each subject lands in a distinct
risk band through the real analyze pipeline:

  real-low   -> a dormant cold-storage receiver (few receives, holds)      -> LOW
  real-med   -> a moderately active hub (some fan-out + a short chain)     -> MEDIUM
  real-high  -> a high-velocity peeling-chain + fan-out mixer-like spender -> HIGH/CRIT

Each case also carries real worldwide network observations (country/ASN pins)
so the Geo Map / Network screens are populated.

NDJSON schema (one object per line) matches ingestion/parse_json.go:
  {"txid","timestamp","fee_btc","inputs":[{address,amount_btc}],
   "outputs":[{address,amount_btc}], "network_observations":[...]}

Writes datasets + a manifest (subjects_real.txt) to argv[1] (default
/tmp/bctx-real).
"""
import datetime
import hashlib
import json
import os
import sys

_NOW = datetime.datetime.now(datetime.timezone.utc)


def recent_ts(hours_back):
    t = _NOW - datetime.timedelta(hours=hours_back)
    return t.strftime("%Y-%m-%dT%H:%M:%SZ")


def txid_for(seed):
    """A deterministic 64-hex txid from a seed string (looks like a real txid)."""
    return hashlib.sha256(seed.encode()).hexdigest()


def tx(seed, ins, outs, hours_back, fee=0.00002):
    return {
        "txid": txid_for(seed),
        "timestamp": recent_ts(hours_back),
        "fee_btc": fee,
        "inputs": [{"address": a, "amount_btc": round(v, 8)} for a, v in ins],
        "outputs": [{"address": a, "amount_btc": round(v, 8)} for a, v in outs],
    }


def write(path, rows):
    with open(path, "w") as f:
        for r in rows:
            f.write(json.dumps(r) + "\n")
    print("wrote %d txs -> %s" % (len(rows), path))


# --- REAL mainnet addresses (public, famous; used here as subjects only) ------
# 1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF : the "Mt. Gox / 1Feex" ~79,957 BTC address.
# 12cbQLTFMXRnSzktFkuoG3eHoMeFtpTu3S : an early high-balance P2PKH address.
# bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97 : Bitfinex-era
#   bech32 cold-wallet style address.
ADDR_LOW = "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF"
ADDR_MED = "12cbQLTFMXRnSzktFkuoG3eHoMeFtpTu3S"
ADDR_HIGH = "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"

# Counterparty addresses (also real-format; cosmetic only).
CP = [
    "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
    "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
    "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
    "1LdRcdxfbSnmCYYNdeYpUnztiYzVfBEQeC",
    "3FkenCiXpSLqD8L79intRNXUgjRoH9sjXa",
    "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
]


def gen_low(d):
    """Dormant cold-storage: a SINGLE pure receive, held. No onward spend, no
    chain, no fan-out, no co-spend — so the deterministic structural detectors
    all read 0 and the lone saturated ML anomaly is damped to the uncorroborated
    floor. This is the genuinely benign shape -> LOW band."""
    return [
        tx("low-single",
           [("1SenderColdStorageXXXXXXXXXXXXaaa", 12.5)],
           [(ADDR_LOW, 12.49)],
           hours_back=72),
    ]


def gen_med(d):
    """Moderately active hub: a receive then a single modest fan-out to a few
    recipients. There is SOME structure (a small spend/distribution) but no long
    peeling chain, no high-velocity burst and no mixing — so it corroborates
    only mildly and lands in the MEDIUM band, between the dormant receiver and
    the high-velocity spender."""
    rows = []
    rows.append(tx("med-recv", [(CP[0], 8.0)], [(ADDR_MED, 7.99)], hours_back=60))
    # one modest fan-out (5 outputs) — a distribution, not a mixer burst.
    outs = [(CP[(i + 1) % len(CP)], 1.5) for i in range(5)]
    rows.append(tx("med-fanout", [(ADDR_MED, 7.99)], outs, hours_back=48))
    return rows


def gen_high(d):
    """High-velocity: a long peeling chain (>=6 hops, high retained value, small
    peels) PLUS a wide fan-out burst in a short window. Strong structural +
    anomaly corroboration -> HIGH / CRITICAL band."""
    rows = []
    # long peeling chain off the subject (8 hops)
    prev, amt = ADDR_HIGH, 50.0
    for i in range(8):
        keep = "bc1qhighpeel%02dkeepxxxxxxxxxxxxxxxxxxxxxx" % i
        peel = "bc1qhighpeel%02dpeelxxxxxxxxxxxxxxxxxxxxxx" % i
        rows.append(tx("high-peel-%d" % i, [(prev, amt)],
                       [(keep, amt - 0.4), (peel, 0.39)], hours_back=18 - i))
        prev, amt = keep, amt - 0.4
    # wide fan-out bursts in a short window (velocity + fan_out)
    for t in range(3):
        outs = [("bc1qhighfan%02d%02dxxxxxxxxxxxxxxxxxxxxxxx" % (t, j), 0.2)
                for j in range(14)]
        rows.append(tx("high-fan-%d" % t, [(ADDR_HIGH, 3.0)], outs,
                       hours_back=6 - t))
    return rows


GEO_ENDPOINTS = [
    ("8.8.8.8", "151.101.1.69", "US", "AS15169"),
    ("1.1.1.1", "104.16.0.1", "AU", "AS13335"),
    ("77.88.8.8", "77.88.8.1", "RU", "AS13238"),
    ("114.114.114.114", "114.114.115.115", "CN", "AS24151"),
    ("168.95.1.1", "168.95.192.1", "TW", "AS3462"),
    ("200.221.11.100", "200.221.11.101", "BR", "AS7738"),
    ("196.25.1.1", "196.25.1.2", "ZA", "AS2018"),
    ("202.12.27.33", "202.12.29.1", "JP", "AS7500"),
    ("91.239.100.100", "89.233.43.71", "DK", "AS51092"),
    ("80.80.80.80", "80.80.81.81", "NL", "AS6830"),
]


def gen_geo(d, case, anchor_txid):
    rows = []
    for i, (src, dst, cc, asn) in enumerate(GEO_ENDPOINTS):
        rows.append({
            "txid": anchor_txid,
            "timestamp": recent_ts(i + 1),
            "network_observations": [{
                "src_ip": src, "src_port": 8333, "dst_ip": dst,
                "dst_port": 8333 + i, "country": cc, "asn": asn,
            }],
        })
    path = os.path.join(d, "%s-geo.ndjson" % case)
    write(path, rows)
    return os.path.basename(path)


def main():
    d = sys.argv[1] if len(sys.argv) > 1 else "/tmp/bctx-real"
    os.makedirs(d, exist_ok=True)

    cases = [
        ("real-low", "real-low.ndjson", ADDR_LOW, gen_low, "low-0"),
        ("real-med", "real-med.ndjson", ADDR_MED, gen_med, "med-recv"),
        ("real-high", "real-high.ndjson", ADDR_HIGH, gen_high, "high-peel-0"),
    ]
    manifest = []
    for case, fname, subject, gen, anchor_seed in cases:
        rows = gen(d)
        write(os.path.join(d, fname), rows)
        geo = gen_geo(d, case, txid_for(anchor_seed))
        manifest.append("%s %s %s %s" % (case, fname, subject, geo))

    with open(os.path.join(d, "subjects_real.txt"), "w") as f:
        f.write("\n".join(manifest) + "\n")
    print("manifest -> %s/subjects_real.txt" % d)


if __name__ == "__main__":
    main()
