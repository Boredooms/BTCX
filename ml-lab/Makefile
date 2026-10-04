# BCTX ML lab — reproducible local pipeline.
# All targets use the sudo-free venv. Nothing downloads at inference time.

PY := .venv/bin/python

.PHONY: env audit freeze anomaly entity flow report offline all clean-venv

env:
	bash scripts/setup_env.sh

freeze:
	$(PY) -m ml_lab.features.freeze

audit:
	$(PY) -m ml_lab.audit.datasets

anomaly:
	$(PY) -m ml_lab.training.anomaly.train
	$(PY) -m ml_lab.evaluation.anomaly
	$(PY) -m ml_lab.export.anomaly_onnx

entity:
	$(PY) -m ml_lab.training.entity.train
	$(PY) -m ml_lab.evaluation.entity

flow:
	$(PY) -m ml_lab.training.flow.train
	$(PY) -m ml_lab.evaluation.flow
	$(PY) -m ml_lab.export.flow_onnx

offline:
	unshare -rn $(PY) -m ml_lab.offline.verify || $(PY) -m ml_lab.offline.verify

report:
	$(PY) -m ml_lab.report

# Full pipeline in gate order.
all: freeze audit anomaly entity flow offline report

clean-venv:
	rm -rf .venv
