#!/usr/bin/env python3
"""Generate a BCTX JSON dataset of network observations keyed to real public
IPs across the world, so the Geo Map resolves them (via the installed GeoIP
City DB) to real country/city coordinates and plots real pins.

The IPs below are well-known public anycast/DNS/service endpoints — real,
routable, and present in GeoLite2-City — used here ONLY as representative
network-telemetry endpoints for the demo map (metadata, never ownership).

Usage: python3 scripts/gen_geo_dataset.py <out.ndjson> [txid]
"""
import datetime
import json
import sys

# (src_ip, dst_ip, country, asn, label) — a geographically diverse set.
ENDPOINTS = [
    ("8.8.8.8",        "151.101.1.69",  "US", "AS15169", "Google (Mountain View)"),
    ("1.1.1.1",        "104.16.0.1",    "AU", "AS13335", "Cloudflare (Sydney/anycast)"),
    ("9.9.9.9",        "199.9.14.201",  "US", "AS19281", "Quad9 (Berkeley)"),
    ("208.67.222.222", "208.67.220.220","US", "AS36692", "OpenDNS"),
    ("195.46.39.39",   "195.46.39.40",  "RU", "AS8359",  "SafeDNS (Moscow)"),
    ("80.80.80.80",    "80.80.81.81",   "NL", "AS6830",  "Freenom (Amsterdam)"),
    ("185.228.168.9",  "185.228.169.9", "GB", "AS205157","CleanBrowsing (London)"),
    ("77.88.8.8",      "77.88.8.1",     "RU", "AS13238", "Yandex (Moscow)"),
    ("114.114.114.114","114.114.115.115","CN","AS24151", "114DNS (Nanjing)"),
    ("168.95.1.1",     "168.95.192.1",  "TW", "AS3462",  "HiNet (Taipei)"),
    ("139.130.4.4",    "203.50.2.71",   "AU", "AS1221",  "Telstra (Melbourne)"),
    ("200.221.11.100", "200.221.11.101","BR", "AS7738",  "UOL (Sao Paulo)"),
    ("196.25.1.1",     "196.25.1.2",    "ZA", "AS2018",  "Telkom (Johannesburg)"),
    ("202.12.27.33",   "202.12.29.1",   "JP", "AS7500",  "WIDE (Tokyo)"),
    ("91.239.100.100", "89.233.43.71",  "DK", "AS51092", "UncensoredDNS (Copenhagen)"),
    ("156.154.70.1",   "156.154.71.1",  "US", "AS36692", "Neustar (DC)"),
]


def main():
    if len(sys.argv) < 2:
        print("usage: gen_geo_dataset.py <out.ndjson> [txid]", file=sys.stderr)
        sys.exit(2)
    out = sys.argv[1]
    txid = sys.argv[2] if len(sys.argv) > 2 else "demo-geo-tx-0001"
    now = datetime.datetime.now(datetime.timezone.utc)
    with open(out, "w") as f:
        for i, (src, dst, cc, asn, _label) in enumerate(ENDPOINTS):
            ts = (now - datetime.timedelta(hours=i + 1)).strftime("%Y-%m-%dT%H:%M:%SZ")
            obj = {
                "txid": txid,
                "timestamp": ts,
                "network_observations": [
                    {
                        "src_ip": src,
                        "src_port": 8333,
                        "dst_ip": dst,
                        "dst_port": 8333 + i,
                        "country": cc,
                        "asn": asn,
                    }
                ],
            }
            f.write(json.dumps(obj) + "\n")
    print(f"wrote {len(ENDPOINTS)} observation records to {out} (txid={txid})")


if __name__ == "__main__":
    main()
