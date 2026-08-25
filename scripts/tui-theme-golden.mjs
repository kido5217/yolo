// Golden-matrix oracle for internal/tui/theme. Ports the upstream PURE
// resolution functions (packages/tui/src/theme/index.ts resolveTheme +
// generateSystem + @opentui/core 0.4.5 RGBA) so the Go port is verified
// bit-for-bit against upstream. Run at repo root:
//   node scripts/tui-theme-golden.mjs
// Writes internal/tui/theme/testdata/theme-golden.json (checked in).
import { readdirSync, readFileSync, writeFileSync } from "node:fs";

// --- @opentui/core 0.4.5 RGBA (int 0-255 representation; bit-identical) ---
function hexToRgb(hex) {
  hex = hex.replace(/^#/, "");
  if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
  else if (hex.length === 4) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2] + hex[3] + hex[3];
  if (!/^[0-9A-Fa-f]{6}$/.test(hex) && !/^[0-9A-Fa-f]{8}$/.test(hex)) return [255, 0, 255, 255]; // upstream: magenta + console.warn
  const r = parseInt(hex.slice(0, 2), 16), g = parseInt(hex.slice(2, 4), 16), b = parseInt(hex.slice(4, 6), 16);
  const a = hex.length === 8 ? parseInt(hex.slice(6, 8), 16) : 255;
  return [r, g, b, a];
}
const toByte = (v) => Math.round(Math.max(0, Math.min(255, Number.isFinite(v) ? v : 0)));
const ANSI_16 = ["#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"];
function ansiToRgba(code) {
  if (code < 16) return hexToRgb(ANSI_16[code] ?? "#000000");
  if (code < 232) {
    const index = code - 16;
    const b = index % 6, g = Math.floor(index / 6) % 6, r = Math.floor(index / 36);
    const val = (x) => (x === 0 ? 0 : x * 40 + 55);
    return [val(r), val(g), val(b), 255];
  }
  if (code < 256) { const gray = (code - 232) * 10 + 8; return [gray, gray, gray, 255]; }
  return [0, 0, 0, 255];
}
const toHex8 = (c) => "#" + c.map((v) => toByte(v).toString(16).padStart(2, "0")).join("");

// --- theme/index.ts resolveTheme (lines 241-299) ---
function resolveTheme(theme, mode) {
  const defs = theme.defs ?? {};
  function resolveColor(c, chain = []) {
    if (Array.isArray(c)) return c; // upstream: c instanceof RGBA
    if (typeof c === "string") {
      if (c === "transparent" || c === "none") return [0, 0, 0, 0];
      if (c.startsWith("#")) return hexToRgb(c);
      if (chain.includes(c)) throw new Error(`Circular color reference: ${[...chain, c].join(" -> ")}`);
      const next = defs[c] ?? theme.theme[c];
      if (next === undefined) throw new Error(`Color reference "${c}" not found in defs or theme`);
      return resolveColor(next, [...chain, c]);
    }
    if (typeof c === "number") return ansiToRgba(c);
    return resolveColor(c[mode], chain);
  }
  const resolved = {};
  for (const [key, value] of Object.entries(theme.theme)) {
    if (key === "selectedListItemText" || key === "backgroundMenu" || key === "thinkingOpacity") continue;
    resolved[key] = resolveColor(value);
  }
  const hasSelectedListItemText = theme.theme.selectedListItemText !== undefined;
  resolved.selectedListItemText = hasSelectedListItemText ? resolveColor(theme.theme.selectedListItemText) : resolved.background;
  resolved.backgroundMenu = theme.theme.backgroundMenu !== undefined ? resolveColor(theme.theme.backgroundMenu) : resolved.backgroundElement;
  return { resolved, thinkingOpacity: theme.theme.thinkingOpacity ?? 0.6, hasSelectedListItemText };
}

// --- theme/index.ts tint (346) + generateGrayScale (471) + generateMutedTextColor (525) + generateSystem (360) + terminalMode (353) ---
// tint preserves the upstream FLOAT 0-1 operation order exactly (base.r +
// (overlay.r - base.r) * alpha, then Math.round(r * 255)) so JS/Go results
// are bit-identical (same IEEE 754 ops).
const tint = (base, overlay, alpha) => [
  Math.round((base[0] / 255 + (overlay[0] / 255 - base[0] / 255) * alpha) * 255),
  Math.round((base[1] / 255 + (overlay[1] / 255 - base[1] / 255) * alpha) * 255),
  Math.round((base[2] / 255 + (overlay[2] / 255 - base[2] / 255) * alpha) * 255),
  255,
];
function generateGrayScale(bg, isDark) {
  const grays = {};
  const bgR = bg[0], bgG = bg[1], bgB = bg[2];
  const luminance = 0.299 * bgR + 0.587 * bgG + 0.114 * bgB;
  for (let i = 1; i <= 12; i++) {
    const factor = i / 12.0;
    let newR, newG, newB;
    if (isDark) {
      if (luminance < 10) {
        const grayValue = Math.floor(factor * 0.4 * 255);
        newR = grayValue; newG = grayValue; newB = grayValue;
      } else {
        const newLum = luminance + (255 - luminance) * factor * 0.4;
        const ratio = newLum / luminance;
        newR = Math.min(bgR * ratio, 255); newG = Math.min(bgG * ratio, 255); newB = Math.min(bgB * ratio, 255);
      }
    } else {
      if (luminance > 245) {
        const grayValue = Math.floor(255 - factor * 0.4 * 255);
        newR = grayValue; newG = grayValue; newB = grayValue;
      } else {
        const newLum = luminance * (1 - factor * 0.4);
        const ratio = newLum / luminance;
        newR = Math.max(bgR * ratio, 0); newG = Math.max(bgG * ratio, 0); newB = Math.max(bgB * ratio, 0);
      }
    }
    grays[i] = [Math.floor(newR), Math.floor(newG), Math.floor(newB), 255];
  }
  return grays;
}
function generateMutedTextColor(bg, isDark) {
  const bgLum = 0.299 * bg[0] + 0.587 * bg[1] + 0.114 * bg[2];
  let grayValue;
  if (isDark) {
    if (bgLum < 10) grayValue = 180;
    else grayValue = Math.min(Math.floor(160 + bgLum * 0.3), 200);
  } else {
    if (bgLum > 245) grayValue = 75;
    else grayValue = Math.max(Math.floor(100 - (255 - bgLum) * 0.2), 60);
  }
  return [grayValue, grayValue, grayValue, 255];
}
// colors: { palette: [16 hex], defaultBackground, defaultForeground } (int form: arrays)
function generateSystem(colors, mode) {
  const bg = colors.defaultBackground ?? hexToRgb(colors.palette[0]);
  const fg = colors.defaultForeground ?? hexToRgb(colors.palette[7]);
  const transparent = [bg[0], bg[1], bg[2], 0];
  const isDark = mode === "dark";
  const col = (i) => (colors.palette[i] ? hexToRgb(colors.palette[i]) : ansiToRgba(i));
  const grays = generateGrayScale(bg, isDark);
  const textMuted = generateMutedTextColor(bg, isDark);
  const ansiColors = { black: col(0), red: col(1), green: col(2), yellow: col(3), blue: col(4), magenta: col(5), cyan: col(6), white: col(7), redBright: col(9), greenBright: col(10) };
  const diffAlpha = isDark ? 0.22 : 0.14;
  const diffAddedBg = tint(bg, ansiColors.green, diffAlpha);
  const diffRemovedBg = tint(bg, ansiColors.red, diffAlpha);
  const diffContextBg = grays[2];
  const diffAddedLineNumberBg = tint(diffContextBg, ansiColors.green, diffAlpha);
  const diffRemovedLineNumberBg = tint(diffContextBg, ansiColors.red, diffAlpha);
  return { theme: {
    primary: ansiColors.cyan, secondary: ansiColors.magenta, accent: ansiColors.cyan,
    error: ansiColors.red, warning: ansiColors.yellow, success: ansiColors.green, info: ansiColors.cyan,
    text: fg, textMuted, selectedListItemText: bg,
    background: transparent, backgroundPanel: grays[2], backgroundElement: grays[3], backgroundMenu: grays[3],
    borderSubtle: grays[6], border: grays[7], borderActive: grays[8],
    diffAdded: ansiColors.green, diffRemoved: ansiColors.red, diffContext: grays[7], diffHunkHeader: grays[7],
    diffHighlightAdded: ansiColors.greenBright, diffHighlightRemoved: ansiColors.redBright,
    diffAddedBg, diffRemovedBg, diffContextBg, diffLineNumber: textMuted,
    diffAddedLineNumberBg, diffRemovedLineNumberBg,
    markdownText: fg, markdownHeading: fg, markdownLink: ansiColors.blue, markdownLinkText: ansiColors.cyan,
    markdownCode: ansiColors.green, markdownBlockQuote: ansiColors.yellow, markdownEmph: ansiColors.yellow,
    markdownStrong: fg, markdownHorizontalRule: grays[7], markdownListItem: ansiColors.blue,
    markdownListEnumeration: ansiColors.cyan, markdownImage: ansiColors.blue, markdownImageText: ansiColors.cyan,
    markdownCodeBlock: fg,
    syntaxComment: textMuted, syntaxKeyword: ansiColors.magenta, syntaxFunction: ansiColors.blue,
    syntaxVariable: fg, syntaxString: ansiColors.green, syntaxNumber: ansiColors.yellow,
    syntaxType: ansiColors.cyan, syntaxOperator: ansiColors.cyan, syntaxPunctuation: fg,
  } };
}
function terminalMode(colors) {
  const bg = colors.defaultBackground;
  if (!bg) return undefined;
  const [r, g, b] = hexToRgb(bg);
  return 0.299 * r + 0.587 * g + 0.114 * b > 0.5 ? "light" : "dark";
}

// --- main ---
const assets = "internal/tui/theme/assets";
const themes = {};
for (const f of readdirSync(assets).sort()) {
  if (!f.endsWith(".json")) continue;
  themes[f.replace(/\.json$/, "")] = JSON.parse(readFileSync(`${assets}/${f}`, "utf8"));
}
const out = {};
for (const [name, tj] of Object.entries(themes)) {
  for (const mode of ["dark", "light"]) {
    const { resolved, thinkingOpacity, hasSelectedListItemText } = resolveTheme(tj, mode);
    const entry = { thinkingOpacity, _hasSelectedListItemText: hasSelectedListItemText };
    for (const [k, v] of Object.entries(resolved)) entry[k] = toHex8(v);
    out[`${name}.${mode}`] = entry;
  }
}
// File 1: the 33x2 matrix (consumed by S0.2's golden test).
writeFileSync("internal/tui/theme/testdata/theme-golden.json", JSON.stringify(out, null, 2) + "\n");
console.log(`wrote internal/tui/theme/testdata/theme-golden.json (${Object.keys(out).length} entries)`);

// File 2: system-theme fixtures (consumed by S0.4): near-black, mid-dark,
// near-white, mid-light backgrounds — each exercises both grays/muted branches.
const XTERM = ["#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"];
const LIGHT16 = ["#000000", "#7f0000", "#007f00", "#7f7f00", "#00007f", "#7f007f", "#007f7f", "#e5e5e5", "#e5e5e5", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff"];
const FIXTURES = {
  "black": { palette: XTERM, defaultBackground: "#000000", defaultForeground: "#ffffff" },
  "mid-dark": { palette: XTERM, defaultBackground: "#1e1e1e", defaultForeground: "#d4d4d4" },
  "white": { palette: LIGHT16, defaultBackground: "#ffffff", defaultForeground: "#000000" },
  "mid-light": { palette: LIGHT16, defaultBackground: "#f0f0f0", defaultForeground: "#1a1a1a" },
};
const sysOut = {};
for (const [name, colors] of Object.entries(FIXTURES)) {
  for (const mode of ["dark", "light"]) {
    const { resolved, thinkingOpacity, hasSelectedListItemText } = resolveTheme(generateSystem(colors, mode), mode);
    const entry = { thinkingOpacity, _hasSelectedListItemText: hasSelectedListItemText };
    for (const [k, v] of Object.entries(resolved)) entry[k] = toHex8(v);
    sysOut[`system.${name}.${mode}`] = entry;
  }
}
writeFileSync("internal/tui/theme/testdata/system-golden.json", JSON.stringify(sysOut, null, 2) + "\n");
console.log(`wrote internal/tui/theme/testdata/system-golden.json (${Object.keys(sysOut).length} entries)`);

// File 3: terminalMode luminance boundaries (consumed by S0.4).
writeFileSync("internal/tui/theme/testdata/terminal-mode-golden.json", JSON.stringify({
  "#000000": terminalMode({ defaultBackground: "#000000" }),
  "#ffffff": terminalMode({ defaultBackground: "#ffffff" }),
  "#7f7f7f": terminalMode({ defaultBackground: "#7f7f7f" }),
  "#808080": terminalMode({ defaultBackground: "#808080" }),
  "missing": terminalMode({}),
}, null, 2) + "\n");
console.log("wrote internal/tui/theme/testdata/terminal-mode-golden.json");
