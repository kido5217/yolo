# CONTEXT.md — yolo domain glossary

## Glossary

- **self-brand** — yolo's self-identity: the strings where yolo refers to
  itself. The prompt identity ("You are YOLO"), the home logo mark
  (the YOLO glyph block), the `--help` text, the keymap labels
  (`yolo.status` / `yolo.debug`), and the default theme name (`yolo`).
  Rebranded to yolo on 2026-09-03 (deviation 265).
- **service identity** — the identity of an external third-party
  service, never of yolo: the OpenCode Zen hosted model catalog
  (provider ids `opencode` / `opencode-go`, the "OpenCode Zen" display
  names, the opencode.ai / models.opencode.ai URLs, `OPENCODE_API_KEY`,
  the catalog's top-level `"opencode"` JSON key) and the opencode.ai
  theme `$schema`. Stays opencode — renaming it would mislabel the
  data source and break the catalog API shape.
- **upstream reference** — the opencode v1.18.18 codebase yolo was
  ported from and verifies against (root principle 2): the provenance
  comments ("ports opencode…"), the plans/specs/reviews history, the
  `x-opencode-directory` upstream header name, and `scripts/parity/`
  (which drives the real upstream binary). Stays opencode.
