# tests/offline — offline proof suite

These tests verify the hard offline requirement: with networking removed, BCTX
must still initialize, open its database, manage cases, and (in later phases)
run graph/ML/risk/evidence/reports.

## Running

```bash
make test-offline
```

This builds the binary and runs core commands under `unshare -rn` (a dropped
network namespace) in `--airgap` mode. Any hidden external call fails hard
because there is literally no network available.

As later phases land, add assertions here for:
- wallet / transaction / IP lookup against local data
- bounded graph expansion
- local ONNX inference
- risk + evidence + explain
- report generation + export
