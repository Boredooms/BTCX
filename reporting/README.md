# reporting/ — reports + exports

Reports are built from a structured `schema.Report`, never by scraping terminal
text. All generation is 100% local.

```
reporting/
├── models/    # report assembly from InvestigationResult
├── markdown/  # Markdown renderer
├── json/      # JSON renderer
├── html/      # html/template renderer
└── pdf/       # local PDF renderer (no SaaS)
```

Implements `sdk.ReportService`. Phase 7.
