import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { apiClient, SignInRequired, type Tokens } from "./api.ts";

// Hands out "token-1", then "token-2" for each fresh request, and counts sign-ins.
function fakeTokens({ refreshFails = false } = {}) {
	let n = 1;
	const tokens: Tokens & { signIns: number; asked: boolean[] } = {
		signIns: 0,
		asked: [],
		async token({ fresh = false } = {}) {
			tokens.asked.push(fresh);
			if (fresh) {
				if (refreshFails) throw new Error("invalid_grant");
				n++;
			}
			return `token-${n}`;
		},
		async signIn() {
			tokens.signIns++;
		},
	};
	return tokens;
}

// Answers each call with the next status, recording the Authorization header.
function fakeFetch(statuses: number[]) {
	const seen: { url: string; auth: string | null; method?: string }[] = [];
	const get = (async (url: string, init?: RequestInit) => {
		seen.push({
			url,
			auth: new Headers(init?.headers).get("Authorization"),
			method: init?.method,
		});
		const status = statuses[Math.min(seen.length - 1, statuses.length - 1)];
		return new Response(status === 204 ? null : "{}", { status });
	}) as typeof fetch;
	return { get, seen };
}

describe("apiClient", () => {
	test("sends the access token to the API", async () => {
		const { get, seen } = fakeFetch([200]);
		const api = apiClient("https://api.test", fakeTokens(), get);
		const res = await api("/api/admin/posts", { method: "POST" });
		assert.equal(res.status, 200);
		assert.deepEqual(seen, [
			{
				url: "https://api.test/api/admin/posts",
				auth: "Bearer token-1",
				method: "POST",
			},
		]);
	});

	test("on a 401 it gets a fresh token once and tries again", async () => {
		const { get, seen } = fakeFetch([401, 200]);
		const tokens = fakeTokens();
		const res = await apiClient("https://api.test", tokens, get)("/x");
		assert.equal(res.status, 200);
		assert.deepEqual(
			seen.map((s) => s.auth),
			["Bearer token-1", "Bearer token-2"],
		);
		assert.deepEqual(tokens.asked, [false, true]);
		assert.equal(tokens.signIns, 0);
	});

	test("a second 401 sends the user to sign in", async () => {
		const { get, seen } = fakeFetch([401, 401]);
		const tokens = fakeTokens();
		await assert.rejects(
			apiClient("https://api.test", tokens, get)("/x"),
			SignInRequired,
		);
		assert.equal(seen.length, 2);
		assert.equal(tokens.signIns, 1);
	});

	test("a refresh that fails sends the user to sign in", async () => {
		const { get } = fakeFetch([401]);
		const tokens = fakeTokens({ refreshFails: true });
		await assert.rejects(
			apiClient("https://api.test", tokens, get)("/x"),
			SignInRequired,
		);
		assert.equal(tokens.signIns, 1);
	});

	test("other answers, 403 included, go back to the caller", async () => {
		const { get, seen } = fakeFetch([403]);
		const res = await apiClient("https://api.test", fakeTokens(), get)("/x");
		assert.equal(res.status, 403);
		assert.equal(seen.length, 1);
	});
});
