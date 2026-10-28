// Generates src/styles/tokens.css from design-system/tokens.json.
// Run with `npm run tokens` after changing a token. Never edit tokens.css by hand.
import { readFileSync, writeFileSync } from "node:fs";

const tokens = JSON.parse(
	readFileSync(
		new URL("../design-system/tokens.json", import.meta.url),
		"utf8",
	),
);
const out = [];
const alias = (value) => value.replace(/^\{(.+)\}$/, "var(--$1)");

out.push(
	"/* Generated from design-system/tokens.json by scripts/build-tokens.mjs. Edit the JSON, then run `npm run tokens`. */",
	"",
);

for (const weight of [400, 500, 600]) {
	out.push(
		"@font-face {",
		'\tfont-family: "Atkinson Hyperlegible Next";',
		`\tsrc: url("/fonts/atkinson-hyperlegible-next-latin-${weight}-normal.woff2") format("woff2");`,
		`\tfont-weight: ${weight};`,
		"\tfont-style: normal;",
		"\tfont-display: swap;",
		"}",
		"",
	);
}

// The raw tokens, as CSS custom properties.
out.push(":root {");
for (const t of tokens.color.tokens)
	out.push(`\t--${t.name}: ${alias(t.value)};`);
for (const family of ["spacing", "layout"]) {
	for (const t of tokens[family].tokens) out.push(`\t--${t.name}: ${t.value};`);
}
out.push("}", "");

// Turn off Tailwind's default scales so only the club's tokens exist as utilities.
out.push(
	"@theme {",
	"\t--color-*: initial;",
	"\t--font-*: initial;",
	"\t--text-*: initial;",
	"\t--font-weight-*: initial;",
	"\t--radius-*: initial;",
	"\t--shadow-*: initial;",
	"\t--inset-shadow-*: initial;",
	"\t--drop-shadow-*: initial;",
	"\t--container-*: initial;",
	...tokens.radius.tokens.map((t) => `\t--${t.name}: ${t.value};`),
	...Object.entries(tokens.type.families).map(
		([key, stack]) => `\t--font-${key}: ${stack};`,
	),
	"}",
	"",
);

// Register the tokens with Tailwind: bg-primary, text-heading-xl, rounded-lg, max-w-content and so on.
out.push("@theme inline {");
for (const t of tokens.color.tokens)
	out.push(`\t--color-${t.name}: var(--${t.name});`);
out.push(
	"\t--font-weight-normal: 400;",
	"\t--font-weight-medium: 500;",
	"\t--font-weight-semibold: 600;",
);
for (const group of tokens.type.groups) {
	for (const s of group.styles) {
		out.push(`\t--text-${s.name}: ${s.fontSize};`);
		out.push(`\t--text-${s.name}--line-height: ${s.lineHeight};`);
		out.push(`\t--text-${s.name}--font-weight: ${s.fontWeight};`);
	}
}
const containers = {
	container: "content",
	measure: "measure",
	"form-measure": "form",
};
for (const [token, name] of Object.entries(containers))
	out.push(`\t--container-${name}: var(--${token});`);
out.push("}", "");

writeFileSync(
	new URL("../src/styles/tokens.css", import.meta.url),
	out.join("\n"),
);
console.log("Wrote src/styles/tokens.css");
