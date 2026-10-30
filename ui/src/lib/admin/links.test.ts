import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { linkAddress } from "./links.ts";

describe("linkAddress", () => {
	test("a web address gets https:// if it was left off", () => {
		assert.equal(
			linkAddress("example.org/fixtures"),
			"https://example.org/fixtures",
		);
		assert.equal(
			linkAddress("  https://example.org  "),
			"https://example.org/",
		);
		assert.equal(linkAddress("http://example.org"), "http://example.org/");
	});

	test("an email address becomes a mailto: link", () => {
		assert.equal(
			linkAddress("secretary@example.org"),
			"mailto:secretary@example.org",
		);
		assert.equal(linkAddress("mailto:a@example.org"), "mailto:a@example.org");
	});

	test("anything else is refused", () => {
		for (const bad of [
			"",
			"javascript:alert(1)",
			"data:text/html,hi",
			"not a link",
			"ftp://example.org",
		]) {
			assert.equal(linkAddress(bad), null, bad);
		}
	});
});
