# Offline Inference Proof

offline_pass: **True**
Socket guard blocks all AF_INET/AF_INET6 connects for this process; only local files are read.

- **anomaly**: {'status': 'ok', 'n': 12, 'sample_scores': [0.11727725714445114, 0.17316558957099915, 0.0]}
- **flow**: {'status': 'ok', 'n': 5, 'sample_predictions': ['normal', 'normal', 'normal', 'mixing_like', 'peeling_chain']}
- **entity**: {'status': 'ok', 'algorithm': 'agglomerative_d6.0', 'features': 12, 'onnx': False, 'note': 'frozen deterministic clustering (no ONNX by design)'}
