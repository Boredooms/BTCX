#!/usr/bin/env python3
"""Generate a suite of BCTX dataset JSON files that exercise DIFFERENT risk
bands and a real entity cluster, so the TUI shows a spread of ML risk scores
(not everything CRITICAL) and the Entity Cluster screen is populated.

Each file is NDJSON (one canonical object per line) with txid + inputs/outputs.
A multi-input tx (>=2 distinct input addresses) is what the common-input
heuristic turns into an entity cluster.

Writes to the directory given as argv[1] (default /tmp/bctx-suite).
"""
import datetime
import json
import os
import sys

# Base "now" for recent timestamps: step i transactions back from recent so the
# dashboard LIVE ACTIVITY feed shows CURRENT dates (not a frozen 2024). Uses the
# real system clock so a 2026 machine shows 2026 dates.
_NOW = datetime.datetime.now(datetime.timezone.utc)


def recent_ts(step):
    """A timestamp `step` hours before now, RFC3339 UTC. step 0 = most recent."""
    t = _NOW - datetime.timedelta(hours=step)
    return t.strftime("%Y-%m-%dT%H:%M:%SZ")


def tx(txid, ins, outs, ts):
    return {
        "txid": txid,
        "timestamp": ts,
        "fee_btc": 0.00002,
        "inputs": [{"address": a, "amount_btc": v} for a, v in ins],
        "outputs": [{"address": a, "amount_btc": v} for a, v in outs],
    }


def write(path, rows):
    with open(path, "w") as f:
        for r in rows:
            f.write(json.dumps(r) + "\n")
    print("wrote %d txs -> %s" % (len(rows), path))


def gen_low(d):
    # Dormant receiver: the subject only RECEIVES a couple of payments from
    # distinct senders and holds (it is an OUTPUT, never an input, so no chain,
    # no fan-out, no peeling, no co-spend). This is the benign/low-activity
    # shape — the flow detector finds no suspicious pattern -> LOW band.
    S = "bc1qlowsubjectaaaaaaaaaaaaaaaaaaaaaaaaa0"
    # A single receive: one sender funds the subject once. No chain, no onward
    # spend, no fan-out, no co-spend — the minimal benign shape.
    rows = [
        tx("lowtx00", [("bc1qlowsenderA0000000000000000000000000", 0.5)],
           [(S, 0.49)], recent_ts(6)),
    ]
    write(os.path.join(d, "low.ndjson"), rows)
    return S


def gen_elevated(d):
    # Peeling chain: a long chain that peels a small amount each hop and keeps
    # the bulk -> ELEVATED (peeling_chain-like).
    S = "bc1qpeelsubjectbbbbbbbbbbbbbbbbbbbbbbbb0"
    rows = []
    prev, amt = S, 5.0
    for i in range(8):
        keep = "bc1qpeelkeep%02dbbbbbbbbbbbbbbbbbbbbbbb" % i
        peel = "bc1qpeeldrop%02dbbbbbbbbbbbbbbbbbbbbbbb" % i
        rows.append(tx("peeltx%02d" % i, [(prev, amt)],
                       [(keep, amt - 0.05), (peel, 0.045)], recent_ts(8 - i)))
        prev, amt = keep, amt - 0.05
    write(os.path.join(d, "elevated.ndjson"), rows)
    return S


def gen_high(d):
    # High fan-out: one subject spends into many outputs in one tx, repeatedly
    # -> HIGH (high_fan_out + anomaly).
    S = "bc1qfanoutsubjectcccccccccccccccccccc0"
    rows = []
    for t in range(4):
        outs = [("bc1qfanleaf%02d%02dccccccccccccccccccc" % (t, j), 0.1) for j in range(12)]
        rows.append(tx("fanouttx%02d" % t, [(S, 1.3)], outs, recent_ts(4 - t)))
    write(os.path.join(d, "high.ndjson"), rows)
    return S


def gen_cluster(d):
    # Co-spend: the subject appears as an INPUT alongside two OTHER addresses in
    # the same transactions -> the common-input heuristic infers a 3-member
    # entity cluster. Repeated across txs to raise confidence.
    S = "bc1qclustersubjectdddddddddddddddddddd0"
    A = "bc1qclusterpeer1ddddddddddddddddddddddd"
    B = "bc1qclusterpeer2ddddddddddddddddddddddd"
    rows = []
    for i in range(5):
        dst = "bc1qclusterdst%02ddddddddddddddddddddd" % i
        rows.append(tx("clustertx%02d" % i,
                       [(S, 0.5), (A, 0.3), (B, 0.2)],      # 3 co-inputs
                       [(dst, 0.99)], recent_ts(5 - i)))
    write(os.path.join(d, "cluster.ndjson"), rows)
    return S


def gen_mixing(d):
    # Many-in / many-out with high entropy -> mixing-like (CRITICAL-ish when
    # corroborated by fan-in + fan-out).
    S = "bc1qmixsubjecteeeeeeeeeeeeeeeeeeeeeeee0"
    rows = []
    for t in range(3):
        ins = [(S, 0.4)] + [("bc1qmixin%02d%02deeeeeeeeeeeeeeeeeeee" % (t, j), 0.4) for j in range(7)]
        outs = [("bc1qmixout%02d%02deeeeeeeeeeeeeeeeeee" % (t, j), 0.39) for j in range(8)]
        rows.append(tx("mixtx%02d" % t, ins, outs, recent_ts(3 - t)))
    write(os.path.join(d, "mixing.ndjson"), rows)
    return S


# Real worldwide public IPs (resolve via the installed GeoLite2-City DB) used as
# representative network-telemetry endpoints so every case's Geo Map / Network
# screen is populated with real country/city pins. Metadata only, never
# ownership (matches scripts/gen_geo_dataset.py's set).
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


def gen_geo(d, case, txid):
    """Per-case network observations keyed to one of the case's txids, so the
    Geo Map / Network screens are populated for THAT case."""
    rows = []
    for i, (src, dst, cc, asn) in enumerate(GEO_ENDPOINTS):
        rows.append({
            "txid": txid,
            "timestamp": recent_ts(i + 1),
            "network_observations": [{
                "src_ip": src, "src_port": 8333, "dst_ip": dst,
                "dst_port": 8333 + i, "country": cc, "asn": asn,
            }],
        })
    path = os.path.join(d, "%s-geo.ndjson" % case)
    with open(path, "w") as f:
        for r in rows:
            f.write(json.dumps(r) + "\n")
    return os.path.basename(path)


def main():
    d = sys.argv[1] if len(sys.argv) > 1 else "/tmp/bctx-suite"
    os.makedirs(d, exist_ok=True)
    subjects = {
        "demo-low": gen_low(d),
        "demo-elevated": gen_elevated(d),
        "demo-high": gen_high(d),
        "demo-cluster": gen_cluster(d),
        "demo-mixing": gen_mixing(d),
    }
    files = {"demo-low": "low", "demo-elevated": "elevated", "demo-high": "high",
             "demo-cluster": "cluster", "demo-mixing": "mixing"}
    # The txid each case's geo observations attach to (the first tx of each set).
    first_tx = {"demo-low": "lowtx00", "demo-elevated": "peeltx00",
                "demo-high": "fanouttx00", "demo-cluster": "clustertx00",
                "demo-mixing": "mixtx00"}
    # Emit a manifest the shell reads: case -> dataset -> subject -> geo-file.
    with open(os.path.join(d, "subjects.txt"), "w") as f:
        for case, subj in subjects.items():
            geo = gen_geo(d, case, first_tx[case])
            f.write("%s %s.ndjson %s %s\n" % (case, files[case], subj, geo))
    print("subjects manifest -> %s/subjects.txt" % d)


if __name__ == "__main__":
    main()
