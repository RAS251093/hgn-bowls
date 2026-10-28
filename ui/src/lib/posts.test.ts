import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { loadPosts, postDate } from "./posts.ts";

const apiPost = {
	id: "01JA",
	slug: "open-day",
	title: "Open day",
	description: "Come along",
	bodyMd: "Hello",
	cover: {
		key: "posts/2026/04/a.jpg",
		url: "https://storage.googleapis.com/b/posts/a.jpg?sig",
		alt: "The green",
	},
	publishedAt: "2026-04-02T09:00:00.000000Z",
	updatedAt: "2026-04-02T09:00:00.000000Z",
};

function fakeFetch(
	status: number,
	body: unknown,
	seen: { url?: string; auth?: string } = {},
) {
	return async (url: string | URL | Request, init?: RequestInit) => {
		seen.url = String(url);
		seen.auth = new Headers(init?.headers).get("Authorization") ?? "";
		return new Response(JSON.stringify(body), { status });
	};
}

describe("loadPosts", () => {
	test("reads published posts from the API with the build token", async () => {
		const seen: { url?: string; auth?: string } = {};
		const posts = await loadPosts({
			apiUrl: "https://api.example",
			buildToken: "t0k",
			onPages: true,
			fetch: fakeFetch(200, { posts: [apiPost] }, seen),
		});
		assert.equal(seen.url, "https://api.example/api/posts");
		assert.equal(seen.auth, "Bearer t0k");
		assert.equal(posts.length, 1);
		assert.deepEqual(posts[0].cover, {
			url: apiPost.cover.url,
			alt: "The green",
		});
		assert.equal(posts[0].slug, "open-day");
	});

	test("a post without a cover has none (the page falls back to the club photo)", async () => {
		const posts = await loadPosts({
			apiUrl: "https://api.example",
			buildToken: "t0k",
			onPages: false,
			fetch: fakeFetch(200, { posts: [{ ...apiPost, cover: null }] }),
		});
		assert.equal(posts[0].cover, null);
	});

	test("without a token, local and CI builds use the sample posts", async () => {
		const posts = await loadPosts({
			apiUrl: "https://api.example",
			buildToken: "",
			onPages: false,
			fetch: fakeFetch(500, {}),
		});
		assert.ok(posts.length >= 3);
		assert.ok(posts.every((p) => p.slug && p.title && p.publishedAt));
	});

	test("on Cloudflare Pages, a missing token fails the build instead of publishing samples", async () => {
		await assert.rejects(
			loadPosts({
				apiUrl: "https://api.example",
				buildToken: "",
				onPages: true,
				fetch: fakeFetch(200, { posts: [] }),
			}),
			/BUILD_TOKEN/,
		);
	});

	test("an API error fails the build, so the live site stays as it is", async () => {
		await assert.rejects(
			loadPosts({
				apiUrl: "https://api.example",
				buildToken: "t0k",
				onPages: true,
				fetch: fakeFetch(401, { error: "no" }),
			}),
			/401/,
		);
	});

	test("posts come newest first", async () => {
		const older = {
			...apiPost,
			id: "old",
			slug: "old",
			publishedAt: "2026-01-01T09:00:00.000000Z",
		};
		const posts = await loadPosts({
			apiUrl: "https://api.example",
			buildToken: "t0k",
			onPages: false,
			fetch: fakeFetch(200, { posts: [older, apiPost] }),
		});
		assert.deepEqual(
			posts.map((p) => p.slug),
			["open-day", "old"],
		);
	});
});

describe("postDate", () => {
	const now = new Date("2026-11-01T10:00:00Z");
	test("written out in UK time, with the year only when it isn't this year", () => {
		assert.equal(
			postDate("2026-04-02T23:30:00.000000Z", now),
			"Friday 3 April",
		);
		assert.equal(
			postDate("2025-12-24T10:00:00.000000Z", now),
			"Wednesday 24 December 2025",
		);
	});
});
