import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { UploadError, uploadImage } from "./upload.ts";

const file = (type: string, size: number) =>
	new File([new Uint8Array(size)], "photo", { type });

interface Seen {
	url: string;
	method?: string;
	headers: Record<string, string>;
	body?: unknown;
}

// The API answers POST /api/admin/uploads; storage answers the PUT.
function fakes({
	api = { status: 200, json: {} as object },
	storage = 200,
}: {
	api?: { status: number; json: object };
	storage?: number | "offline";
} = {}) {
	const seen: Seen[] = [];
	const record = (url: string, init: RequestInit = {}) =>
		seen.push({
			url,
			method: init.method,
			headers: Object.fromEntries(new Headers(init.headers)),
			body: init.body,
		});
	const callApi = async (path: string, init?: RequestInit) => {
		record(path, init);
		return new Response(JSON.stringify(api.json), { status: api.status });
	};
	const put = (async (url: string, init?: RequestInit) => {
		record(url, init);
		if (storage === "offline") throw new TypeError("Failed to fetch");
		return new Response(null, { status: storage });
	}) as typeof fetch;
	return { callApi, put, seen };
}

const signed = {
	key: "posts/2026/10/01ABC.jpg",
	uploadURL:
		"https://storage.googleapis.com/bucket/posts/2026/10/01ABC.jpg?sig",
	headers: {
		"Content-Type": "image/jpeg",
		"x-goog-content-length-range": "0,2048",
	},
};

describe("uploadImage", () => {
	test("asks the API where to upload, then sends the photo straight to storage", async () => {
		const { callApi, put, seen } = fakes({
			api: { status: 200, json: signed },
		});
		const photo = file("image/jpeg", 2048);
		const key = await uploadImage(callApi, photo, put);

		assert.equal(key, "posts/2026/10/01ABC.jpg");
		assert.equal(seen[0].url, "/api/admin/uploads");
		assert.equal(seen[0].method, "POST");
		assert.deepEqual(JSON.parse(String(seen[0].body)), {
			contentType: "image/jpeg",
			size: 2048,
		});
		assert.equal(seen[1].url, signed.uploadURL);
		assert.equal(seen[1].method, "PUT");
		assert.equal(seen[1].headers["content-type"], "image/jpeg");
		assert.equal(seen[1].headers["x-goog-content-length-range"], "0,2048");
		assert.equal(seen[1].body, photo);
		assert.equal(
			seen[1].headers.authorization,
			undefined,
			"sent the admin's token to storage",
		);
	});

	test("refuses other kinds of file before uploading anything", async () => {
		const { callApi, put, seen } = fakes();
		await assert.rejects(uploadImage(callApi, file("image/heic", 10), put), {
			name: "UploadError",
			message: "Choose a JPEG, PNG or WebP photo",
		});
		assert.equal(seen.length, 0);
	});

	test("refuses photos over 10 MB before uploading anything", async () => {
		const { callApi, put, seen } = fakes();
		await assert.rejects(
			uploadImage(callApi, file("image/png", 10 * 1024 * 1024 + 1), put),
			{ message: "Choose a photo of 10 MB or smaller" },
		);
		assert.equal(seen.length, 0);
	});

	test("passes on what the API says is wrong", async () => {
		const { callApi, put } = fakes({
			api: {
				status: 422,
				json: { fields: { size: "Images must be 10 MB or smaller" } },
			},
		});
		await assert.rejects(uploadImage(callApi, file("image/png", 10), put), {
			message: "Images must be 10 MB or smaller",
		});
	});

	test("a failed upload says to try again", async () => {
		for (const storage of [403, "offline"] as const) {
			const { callApi, put } = fakes({
				api: { status: 200, json: signed },
				storage,
			});
			await assert.rejects(
				uploadImage(callApi, file("image/jpeg", 10), put),
				(err) => {
					assert.ok(err instanceof UploadError);
					assert.equal(
						err.message,
						"The photo didn't upload. Check your connection, then try again.",
					);
					return true;
				},
			);
		}
	});

	test("uploads the shrunk photo, with its type and size", async () => {
		const { callApi, put, seen } = fakes({
			api: { status: 200, json: signed },
		});
		const original = file("image/jpeg", 5000);
		const smaller = new File([new Uint8Array(800)], "photo.webp", {
			type: "image/webp",
		});
		await uploadImage(callApi, original, put, async () => smaller);
		assert.deepEqual(JSON.parse(String(seen[0].body)), {
			contentType: "image/webp",
			size: 800,
		});
		assert.equal(seen[1].body, smaller);
	});

	test("checks the type and size of what it would upload, before shrinking", async () => {
		const { callApi, put, seen } = fakes();
		let shrunk = false;
		await assert.rejects(
			uploadImage(callApi, file("image/heic", 10), put, async (f) => {
				shrunk = true;
				return f;
			}),
			{ message: "Choose a JPEG, PNG or WebP photo" },
		);
		assert.equal(shrunk, false);
		assert.equal(seen.length, 0);
	});
});
