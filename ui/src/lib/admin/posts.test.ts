import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { describePosts } from "./posts.ts";

const post = (status: "draft" | "published") => ({ status });

describe("describePosts", () => {
	test("counts published posts and drafts", () => {
		assert.equal(
			describePosts([post("published"), post("published"), post("draft")]),
			"2 published posts and 1 draft",
		);
		assert.equal(
			describePosts([post("published"), post("draft"), post("draft")]),
			"1 published post and 2 drafts",
		);
	});

	test("leaves out a kind there are none of", () => {
		assert.equal(describePosts([post("published")]), "1 published post");
		assert.equal(describePosts([post("draft"), post("draft")]), "2 drafts");
	});

	test("says when there are none", () => {
		assert.equal(describePosts([]), "No posts yet");
	});
});
