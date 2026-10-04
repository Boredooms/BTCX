"""Force IPv4 for all Python socket resolution in this process.

This WSL instance resolves AAAA (IPv6) records but has no working outbound
IPv6 route, so any library that tries IPv6 first (pip, urllib, requests) stalls
until timeout. Placing this module's directory on PYTHONPATH as ``sitecustomize``
makes Python drop IPv6 results at resolution time, so only IPv4 is attempted.

Used only for the setup/install step. It performs no network calls itself.
"""

import socket

_orig_getaddrinfo = socket.getaddrinfo


def _ipv4_only(host, port, family=0, type=0, proto=0, flags=0):
    results = _orig_getaddrinfo(host, port, family, type, proto, flags)
    v4 = [r for r in results if r[0] == socket.AF_INET]
    return v4 or results


socket.getaddrinfo = _ipv4_only
