import json
import sys

d = json.load(open(sys.argv[1] if len(sys.argv) > 1 else "/tmp/w123.json"))
sg = d.get("subgraph") or {}
print("subgraph center:", sg.get("center"), "depth:", sg.get("depth"),
      "nodes:", len(sg.get("nodes") or []), "edges:", len(sg.get("edges") or []))
print("related_wallets:", d.get("related_wallets"),
      "relevant_transactions:", d.get("relevant_transactions"))
risk = d.get("risk") or {}
print("risk:", risk.get("score"), "confidence:", risk.get("confidence"))
