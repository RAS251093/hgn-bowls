import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { fromMarkdown, toMarkdown } from "./markdown.ts";

// The editor shows rich text and saves Markdown, so whatever an admin
// writes must survive Markdown -> editor -> Markdown unchanged.
const roundTrip = (md: string) => toMarkdown(fromMarkdown(md));

describe("editor Markdown", () => {
	for (const [name, md] of [
		["paragraphs", "First paragraph.\n\nSecond paragraph."],
		["subheadings", "## Results\n\nWe won.\n\n### Away match\n\nWe lost."],
		["bold and italic", "A **bold** and an *italic* word."],
		["links", "See [the fixtures](https://example.org/fixtures) for dates."],
		["bullet lists", "- Bring flat shoes\n- Bring a jumper"],
		["numbered lists", "1. Arrive at 1pm\n2. Play at 2pm"],
		["quotes", "> Please don't use the green when it's frosty."],
	]) {
		test(`${name} survive a round trip`, () => {
			assert.equal(roundTrip(md), md);
			// And a second trip changes nothing more.
			assert.equal(roundTrip(roundTrip(md)), md);
		});
	}

	test("the editor's own content survives a round trip", () => {
		const doc = fromMarkdown(
			"## AGM\n\nThe AGM is on **Tuesday**, see [details](https://example.org).\n\n- One\n- Two",
		);
		assert.deepEqual(fromMarkdown(toMarkdown(doc)), doc);
	});

	test("a YouTube embed from the old site becomes a video link, not lost", () => {
		const md = roundTrip(
			'Intro.\n\n<iframe width="560" height="315" src="https://www.youtube.com/embed/LegryDpTHXo?si=abc" allowfullscreen></iframe>\n\nAfter.',
		);
		assert.match(
			md,
			/^Intro\.\n\n.*https:\/\/youtu\.be\/LegryDpTHXo.*\n\nAfter\.$/s,
		);
		assert.doesNotMatch(md, /iframe/);
	});

	test("headings fit the editor's two levels: h1 becomes h2, deeper ones h3", () => {
		assert.equal(roundTrip("# Big news"), "## Big news");
		assert.equal(roundTrip("##### To register"), "### To register");
	});

	test("other raw HTML is dropped, as the site drops it", () => {
		assert.doesNotMatch(
			roundTrip("Hello.\n\n<script>alert(1)</script>\n\nBye."),
			/script/,
		);
	});

	test("empty paragraphs (extra presses of Enter) aren't saved as blank space", () => {
		const doc = {
			type: "doc",
			content: [
				{ type: "paragraph", content: [{ type: "text", text: "One." }] },
				{ type: "paragraph" },
				{ type: "paragraph" },
				{ type: "paragraph", content: [{ type: "text", text: "Two." }] },
				{ type: "paragraph" },
			],
		};
		assert.equal(toMarkdown(doc), "One.\n\nTwo.");
	});
});
