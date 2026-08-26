---
type: concept
title: Provider Registry and Catalogs
description: "internal/provider builds the model/provider catalog (kido live probe with static fallback, the cached opencode zen catalog, and config-defined providers), resolves provider/model refs to a driver, and internal/auth resolves API keys (env, auth.json, config)."
tags: [provider, registry, catalog, zen, kido, auth, resolution, driver]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-b12cba1b5936a1da8739d39c
    resource: repo://internal/auth/auth.go
  - id: openwiki-source-3d24ced7ff03bbe0cd0e825f
    resource: repo://internal/provider/kido.go
  - id: openwiki-source-4f753c814d3ee246401def7c
    resource: repo://internal/provider/provider.go
  - id: openwiki-source-f96f3df3bfdf1465e57d866d
    resource: repo://internal/provider/seams.go
  - id: openwiki-source-11b5c26cf36671d242cde267
    resource: repo://internal/provider/zen.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Provider Registry and Catalogs

`internal/provider` builds the model/provider catalog: **kido** (live probe with
static fallback), the **opencode zen catalog** (cached, filtered), and
**user-defined config providers** (internal/provider/provider.go:1-4). The
session engine resolves a session's `provider/model` ref through the
`Registry` and asks it which `llm.Driver` to use (internal/llm/drivers).
`internal/auth` owns per-provider API-key persistence and resolution.

## The Registry

`New(ctx, cfg, httpc, homeDirs)` builds the registry
(internal/provider/provider.go:102-181):

1. **kido** — `FetchKido` (below), `Source: "builtin"`, no key required. A
   config override (`cfg.Provider["kido"].BaseURL`) replaces the base URL.
2. **opencode (zen)** — best-effort by contract: a failed cache/live load, a
   parse failure, or missing `opencode` metadata simply **omits the opencode
   provider** (yolo keeps working on kido); an empty `opencode.api` falls back
   to the production Zen base. **Startup never fails because of it.**
3. **config-defined providers** — every `cfg.Provider` id other than kido/
   opencode that has a base URL and at least one model, sorted by id
   (`Source: "config"`, key required).

After building, each provider's `KeyLoaded` is set when `auth.ResolveKey` returns
a non-empty key. The default provider/model is `kido/Qwen3.8-27B`, overridden by
`cfg.Model` (a `provider/model` ref).

### Catalog entries

`Model` carries `ID`, `Name`, `Family`, `Adapter` (`"openai"|"anthropic"`),
`ToolCall`/`Reasoning`/`Attachment` booleans, `Context`/`Output`, and the four
costs (USD per 1M) (internal/provider/provider.go:20-27). `Info` is one
provider's catalog state: `ID`, `Name`, `Source`, `BaseURL`,
`KeyRequired`/`KeyLoaded`, `Env`, and `Models`; it maps to `protocol.Provider`
in `List()` (internal/provider/provider.go:29-35, 236-264).

### Catalog locations

`Dirs` defaults (internal/provider/provider.go:46-100): `KidoBase`
`https://ai.kido.ws/v1`, `ZenBase` `https://opencode.ai/zen/v1`, `ZenCatalog`
`https://models.opencode.ai/api.json`, `ZenCache`
`<config.CacheYoloDir>/models.json`. `OverridableDirs` fills empty fields with
the production defaults.

### Resolving to a driver

`Resolve(ref)` (internal/provider/provider.go:277-301) maps a `provider/model`
reference (empty uses the defaults): it first consults the test seam, then finds
the provider by id and the model by id within it, returning
`unknown model` / `unknown provider` errors. `DriverFor(m)` picks the driver by
the model's adapter — `anthropic` → `llm.NewAnthropic`, everything else →
`llm.NewOpenAI` (internal/provider/provider.go:303-309). `authStatus` reports
`not-required`, `loaded`, or `missing` (internal/provider/provider.go:266-275).

## The zen catalog (cached, filtered)

`CatalogPolicy` serves the zen catalog with a **TTL-bounded cache**
(internal/provider/zen.go:92-189): a **fresh cache file wins** (mod time within
the TTL); otherwise it **fetches live** (10 s timeout, a browser-style
`User-Agent`, 8 MiB body cap) and **rewrites the cache atomically** (temp file +
rename, so a crash never leaves a half-written cache); a **failed fetch falls
back to the stale cache**.

`ParseZenCatalog` (internal/provider/zen.go:37-71) **keeps only paid models**
(`cost.input > 0`), **excludes `@ai-sdk/google` providers**, and maps the
adapter from `provider.npm` (`@ai-sdk/anthropic` → anthropic, everything else →
openai); results are sorted by id. `parseZenMeta` reads the top-level
`opencode` `name`/`api`/`env`; an empty `api` falls back to the production Zen
base (internal/provider/zen.go:73-90, 138-143).

## The kido catalog (live probe, static fallback)

`FetchKido` (internal/provider/kido.go:12-77) probes `{base}/models` (llamacpp /
vLLM shape) with a bounded timeout and maps live models (id, `n_ctx` → context,
`output = min(32768, n_ctx/8)`). On `noNet`, a network failure, a bad status, a
parse failure, or an all-skipped result, it returns the **static
`kidoFallback`** (`Qwen3.8-27B`). **Network problems never block or fail
startup**, so the result carries no error.

## Test seams

- `NewWithSeams` builds an **offline registry** whose catalog comes from a seam
  (one synthetic model per provider id) instead of the live catalogs — used by
  the engine tests so they never hit the network
  (internal/provider/seams.go:9-14).
- `NewStaticForTest` builds a **fully offline registry**: `kido/q` (default, no
  key) plus `opencode` (key required) seeded with a minimal zen catalog
  (`claude-opus-4-7` anthropic + `gpt-5-nano` openai); a seed replaces the
  opencode models when given — used by server tests
  (internal/provider/seams.go:16-60).
- `resolveSeam` runs before the normal lookup when a seam is set
  (internal/provider/seams.go:62-75).

## API-key resolution (internal/auth)

`ResolveKeyWithSource` (internal/auth/auth.go:118-140) resolves a provider's
API key in **spec order: env → auth.json → config** (the config path checks
`provider.<id>.apiKey`, then `provider.<id>.options.apiKey`). It also reports the
winning source (`"env"|"auth.json"|"config"`) — the source, never the key
itself, is what gets logged. `EnvName` is the uppercased provider id with
non-alphanumerics replaced by `_` plus `_API_KEY` (internal/auth/auth.go:103-116).

Keys persist in a `Store` (providerID → `Entry`{`Type`, `Key`, `Metadata`}) at
`<DataYoloDir>/auth.json`, written with the directory at `0700` and the file at
`0600`; a missing file is an empty store
(internal/auth/auth.go:15-86).

## Representative tests

- Catalog filtering (paid-only, google exclusion, adapter mapping), cache
  policy (fresh/stale/fetch-fail fallback, atomic write), and kido fallback are
  unit-tested in `internal/provider`.
- Key resolution precedence and the `auth.json` file mode are unit-tested in
  `internal/auth`.
- The offline seams back the engine and server test suites (see the testing
  page).
