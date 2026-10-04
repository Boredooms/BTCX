#!/usr/bin/env python3
# Generate a larger deterministic BCTX seed dataset: many transactions across
# wallets forming peeling chains + fan-out, with worldwide network observations
# (real public IP ranges GeoLite2 resolves across many countries). Pure stdlib,
# no network. Writes JSON array to stdout.
import json, hashlib

# Real, well-known public IPs spread across the globe (resolve in GeoLite2-City).
WORLD_IPS = [
    "8.8.8.8", "1.1.1.1", "9.9.9.9", "208.67.222.222",      # US
    "151.101.1.69", "199.59.148.10",                          # US (Fastly/Twitter)
    "77.88.8.8", "5.255.255.70", "213.180.193.1",             # Russia (Yandex)
    "223.5.5.5", "114.114.114.114", "180.76.76.76",           # China
    "202.12.27.33", "210.140.92.183",                         # Japan
    "200.160.2.3", "177.71.207.1",                            # Brazil
    "41.0.5.5", "105.112.0.1", "196.25.1.1",                  # South Africa / Nigeria
    "1.0.0.1", "103.21.244.1",                                # Australia / APNIC
    "185.60.216.35", "193.0.14.129",                          # Europe (RIPE)
    "203.119.101.61", "202.108.22.5",                         # Asia-Pacific
    "62.149.128.4", "80.80.80.80",                            # Italy / NL
    "168.95.1.1", "211.138.180.2",                            # Taiwan / China
    "190.93.240.1", "189.1.168.1",                            # LatAm
]
ASNS = ["AS15169","AS13335","AS54113","AS13238","AS37963","AS7497","AS22548",
        "AS36873","AS3356","AS174","AS2914","AS6939","AS1299","AS4134"]

def wal(tag, i):
    h = hashlib.sha256(f"{tag}{i}".encode()).hexdigest()[:34]
    return "bc1q" + h

def txid(i):
    return hashlib.sha256(f"tx{i}".encode()).hexdigest()

records = []
ip_i = 0
def next_ips(n):
    global ip_i
    out = []
    for _ in range(n):
        out.append(WORLD_IPS[ip_i % len(WORLD_IPS)])
        ip_i += 1
    return out

tx_n = 0
# 1) A long peeling chain (classic laundering structure) — 12 hops.
amount = 50.0
prev = wal("peel", 0)
for hop in range(12):
    tx_n += 1
    cur_peel = wal("peelout", hop)
    cur_keep = wal("peel", hop + 1)
    peel = round(0.3 + hop * 0.05, 4)
    keep = round(amount - peel - 0.0002, 4)
    ips = next_ips(2)
    records.append({
        "txid": txid(tx_n),
        "timestamp": f"2024-08-21T{10 + hop % 12:02d}:{(hop*7)%60:02d}:00Z",
        "fee_btc": 0.0002, "script_type": "p2wpkh",
        "base_size": 141, "total_size": 222, "weight": 561, "vsize": 141,
        "inputs":  [{"address": prev, "amount_btc": amount}],
        "outputs": [{"address": cur_peel, "amount_btc": peel},
                    {"address": cur_keep, "amount_btc": keep}],
        "network_observations": [
            {"src_ip": ips[0], "src_port": 8333, "dst_ip": ips[1], "dst_port": 8333,
             "asn": ASNS[hop % len(ASNS)]},
        ],
    })
    prev = cur_keep
    amount = keep

# 2) A fan-out (one wallet to many) — mixing-like.
tx_n += 1
hub = wal("hub", 0)
outs = [{"address": wal("leaf", k), "amount_btc": round(0.5 + k*0.1, 4)} for k in range(8)]
ips = next_ips(4)
records.append({
    "txid": txid(tx_n),
    "timestamp": "2024-08-21T15:00:00Z",
    "fee_btc": 0.0005, "script_type": "p2sh",
    "base_size": 400, "total_size": 700, "weight": 1600, "vsize": 400,
    "inputs":  [{"address": prev, "amount_btc": amount}],
    "outputs": outs,
    "network_observations": [
        {"src_ip": ips[0], "src_port": 8333, "dst_ip": ips[1], "dst_port": 8333, "asn": ASNS[2]},
        {"src_ip": ips[2], "src_port": 8333, "dst_ip": ips[3], "dst_port": 8333, "asn": ASNS[5]},
    ],
})

# 3) Fan-in (many wallets to one) — consolidation.
tx_n += 1
ins = [{"address": wal("leaf", k), "amount_btc": round(0.5 + k*0.1 - 0.0001, 4)} for k in range(8)]
ips = next_ips(3)
records.append({
    "txid": txid(tx_n),
    "timestamp": "2024-08-21T16:30:00Z",
    "fee_btc": 0.0006, "script_type": "p2tr",
    "base_size": 450, "total_size": 800, "weight": 1800, "vsize": 450,
    "inputs":  ins,
    "outputs": [{"address": wal("sink", 0), "amount_btc": 6.3}],
    "network_observations": [
        {"src_ip": ips[0], "src_port": 8333, "dst_ip": ips[1], "dst_port": 8333, "asn": ASNS[7]},
        {"src_ip": ips[2], "src_port": 8333, "dst_ip": WORLD_IPS[0], "dst_port": 8333, "asn": ASNS[0]},
    ],
})

# 4) A spread of simple transfers to light up more countries/IPs.
for j in range(10):
    tx_n += 1
    ips = next_ips(2)
    records.append({
        "txid": txid(tx_n),
        "timestamp": f"2024-08-22T{j:02d}:15:00Z",
        "fee_btc": 0.0001, "script_type": "p2wpkh",
        "base_size": 110, "total_size": 150, "weight": 450, "vsize": 113,
        "inputs":  [{"address": wal("rnd", j), "amount_btc": round(1.0 + j*0.3, 4)}],
        "outputs": [{"address": wal("rnd", j+1), "amount_btc": round(1.0 + j*0.3 - 0.0001, 4)}],
        "network_observations": [
            {"src_ip": ips[0], "src_port": 8333, "dst_ip": ips[1], "dst_port": 8333,
             "asn": ASNS[j % len(ASNS)]},
        ],
    })

print(json.dumps(records, indent=1))
