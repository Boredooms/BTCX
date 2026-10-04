// Package html renders a report view to a single, fully self-contained HTML
// file. It is NETWORK-FORBIDDEN: the output embeds all styling inline and
// references NO external assets (no remote CSS/JS, no CDN, no web fonts, no
// remote images). Opening the file on an air-gapped machine works offline.
package html

// styleCSS is the complete stylesheet, inlined into a <style> tag by the
// renderer. It uses only system font stacks (no @import, no url(http...), no
// web fonts) so the document is fully self-contained.
const styleCSS = `
:root {
  --fg: #1a1a1a;
  --muted: #555;
  --bg: #ffffff;
  --accent: #2a4d69;
  --border: #d0d0d0;
  --row: #f6f8fa;
  --mono: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  --sans: system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif;
}
* { box-sizing: border-box; }
body {
  font-family: var(--sans);
  color: var(--fg);
  background: var(--bg);
  margin: 0;
  padding: 2rem;
  line-height: 1.5;
  max-width: 60rem;
}
h1 { font-size: 1.8rem; color: var(--accent); margin: 0 0 1rem; }
h2 {
  font-size: 1.2rem;
  color: var(--accent);
  margin: 2rem 0 0.75rem;
  padding-bottom: 0.25rem;
  border-bottom: 2px solid var(--border);
}
.meta dl, .kv dl { display: grid; grid-template-columns: max-content 1fr; gap: 0.25rem 1rem; }
dt { font-weight: 600; color: var(--muted); }
dd { margin: 0; }
code, .mono { font-family: var(--mono); font-size: 0.9em; word-break: break-all; }
table { border-collapse: collapse; width: 100%; margin: 0.5rem 0; font-size: 0.9rem; }
th, td { border: 1px solid var(--border); padding: 0.4rem 0.6rem; text-align: left; vertical-align: top; }
th { background: var(--row); }
tr:nth-child(even) td { background: var(--row); }
ul { margin: 0.5rem 0; padding-left: 1.25rem; }
.caveat { border-left: 3px solid var(--accent); padding: 0.5rem 0.75rem; background: var(--row); color: var(--muted); margin: 0.75rem 0; }
.na { color: var(--muted); font-style: italic; }
footer { margin-top: 2rem; color: var(--muted); font-size: 0.85rem; border-top: 1px solid var(--border); padding-top: 0.75rem; }
`
