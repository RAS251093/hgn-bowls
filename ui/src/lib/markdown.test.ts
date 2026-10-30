import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { renderPost } from "./markdown.ts";

describe("renderPost", () => {
	test("renders the editor's Markdown", async () => {
		const html = await renderPost(
			"Hello **members**.\n\n## Results\n\n- Won 5-2\n- [Report](https://example.org/r)",
		);
		assert.match(html, /<p>Hello <strong>members<\/strong>\.<\/p>/);
		assert.match(html, /<h2>Results<\/h2>/);
		assert.match(html, /<li>Won 5-2<\/li>/);
		assert.match(html, /<a href="https:\/\/example\.org\/r">Report<\/a>/);
	});

	test("the post title is the page's only h1, so a body h1 becomes h2", async () => {
		const html = await renderPost("# Big news\n\n### Detail");
		assert.match(html, /<h2>Big news<\/h2>/);
		assert.match(html, /<h3>Detail<\/h3>/);
		assert.doesNotMatch(html, /<h1/);
	});

	// Screen reader users jump between headings, so levels mustn't skip.
	// Both imported posts did: h3 straight after the title, h5 after an h2.
	test("headings never skip a level", async () => {
		const html = await renderPost(
			"### Indoor courses\n\n#### Who\n\n## AGM\n\n##### To register\n\n### Maintenance",
		);
		const levels = [...html.matchAll(/<(h\d)>([^<]+)</g)].map(
			(m) => `${m[1]} ${m[2]}`,
		);
		assert.deepEqual(levels, [
			"h2 Indoor courses",
			"h3 Who",
			"h2 AGM",
			"h3 To register",
			"h3 Maintenance",
		]);
	});

	test("a pasted address becomes a link", async () => {
		const html = await renderPost(
			"Watch it here: https://youtu.be/LegryDpTHXo",
		);
		assert.match(
			html,
			/<a href="https:\/\/youtu\.be\/LegryDpTHXo">https:\/\/youtu\.be\/LegryDpTHXo<\/a>/,
		);
	});

	test("tables and strikethrough render", async () => {
		const html = await renderPost(
			"~~Cancelled~~ Moved to Sunday\n\n| Team | Score |\n| --- | --- |\n| Home | 21 |",
		);
		assert.match(html, /<del>Cancelled<\/del>/);
		assert.match(html, /<table>/);
		assert.match(html, /<td>21<\/td>/);
	});

	const attacks: { name: string; md: string; banned: RegExp }[] = [
		{
			name: "a script tag",
			md: "Hi <script>alert(1)</script>",
			banned: /<script|alert\(1\)<\/script/,
		},
		{
			name: "an inline handler",
			md: '<img src="x" onerror="alert(1)">',
			banned: /onerror/,
		},
		{
			name: "an iframe",
			md: '<iframe src="https://evil.example"></iframe>',
			banned: /<iframe/,
		},
		{
			name: "a javascript: link",
			md: "[click](javascript:alert(1))",
			banned: /javascript:/,
		},
		{
			name: "a style attribute",
			md: '<p style="position:fixed">x</p>',
			banned: /style=/,
		},
	];
	for (const a of attacks) {
		test(`removes ${a.name}`, async () => {
			assert.doesNotMatch(await renderPost(a.md), a.banned);
		});
	}
});
