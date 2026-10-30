import assert from "node:assert/strict";
import { describe, mock, test } from "node:test";
import { Autosave, type SaveStatus } from "./autosave.ts";
import { type Draft, validateDraft } from "./draft.ts";

const draft = (d: Partial<Draft> = {}): Draft => ({
	title: "Opening day",
	description: "The green opens.",
	bodyMd: "We opened.",
	coverImageKey: null,
	coverAlt: null,
	...d,
});

interface Call {
	method: string;
	path: string;
	ifMatch: string | null;
	body: Record<string, unknown>;
}

// A fake API answering each call with the next reply.
function fakeApi(replies: { status: number; etag?: string; json?: object }[]) {
	const calls: Call[] = [];
	const api = async (path: string, init: RequestInit = {}) => {
		calls.push({
			method: init.method ?? "GET",
			path,
			ifMatch: new Headers(init.headers).get("If-Match"),
			body: JSON.parse(String(init.body ?? "{}")),
		});
		const r = replies[Math.min(calls.length - 1, replies.length - 1)];
		if (r.status === 0) throw new TypeError("Failed to fetch");
		return new Response(JSON.stringify(r.json ?? {}), {
			status: r.status,
			headers: r.etag ? { ETag: r.etag } : {},
		});
	};
	return { api, calls };
}

function setup(
	replies: Parameters<typeof fakeApi>[0],
	post: { id: string; etag: string } | null = null,
) {
	const { api, calls } = fakeApi(replies);
	const statuses: SaveStatus[] = [];
	const created: string[] = [];
	const saver = new Autosave({
		api,
		post,
		delay: 1000,
		onStatus: (s) => statuses.push(s),
		onCreated: (id) => created.push(id),
	});
	return { saver, calls, statuses, created, last: () => statuses.at(-1) };
}

describe("validateDraft", () => {
	test("a title is needed, within the API's limits", () => {
		assert.deepEqual(validateDraft(draft({ title: "  " })), {
			title: "Add a headline",
		});
		assert.deepEqual(validateDraft(draft({ title: "x".repeat(121) })), {
			title: "Keep the headline to 120 characters or fewer",
		});
		assert.deepEqual(validateDraft(draft({ title: "x".repeat(120) })), {});
	});

	test("the description has a limit too, counted in characters", () => {
		assert.deepEqual(validateDraft(draft({ description: "é".repeat(301) })), {
			description: "Keep the description to 300 characters or fewer",
		});
		assert.deepEqual(
			validateDraft(draft({ description: "é".repeat(300) })),
			{},
		);
	});
});

describe("Autosave", () => {
	test("a new post is created on its first save, then updated with If-Match", async () => {
		const { saver, calls, created, last } = setup([
			{ status: 201, etag: '"v1"', json: { id: "01NEW" } },
			{ status: 200, etag: '"v2"', json: { id: "01NEW" } },
		]);
		saver.update(draft());
		await saver.flush();
		saver.update(draft({ bodyMd: "We opened. It was sunny." }));
		await saver.flush();

		assert.deepEqual(
			calls.map((c) => [c.method, c.path, c.ifMatch]),
			[
				["POST", "/api/admin/posts", null],
				["PUT", "/api/admin/posts/01NEW", '"v1"'],
			],
		);
		assert.equal(calls[1].body.bodyMd, "We opened. It was sunny.");
		assert.deepEqual(created, ["01NEW"]);
		assert.equal(last()?.kind, "saved");
	});

	test("an existing post is saved with the ETag it was loaded with", async () => {
		const { saver, calls } = setup([{ status: 200, etag: '"v8"' }], {
			id: "01OLD",
			etag: '"v7"',
		});
		saver.update(draft());
		await saver.flush();
		assert.equal(calls[0].ifMatch, '"v7"');
	});

	test("the cover is sent back unchanged, so saving never drops it", async () => {
		const { saver, calls } = setup([{ status: 200, etag: '"v2"' }], {
			id: "01OLD",
			etag: '"v1"',
		});
		saver.update(
			draft({ coverImageKey: "posts/01COVER.jpg", coverAlt: "Two bowlers" }),
		);
		await saver.flush();
		assert.equal(calls[0].body.coverImageKey, "posts/01COVER.jpg");
		assert.equal(calls[0].body.coverAlt, "Two bowlers");
	});

	test("it waits for a pause in typing, then saves once", async () => {
		mock.timers.enable({ apis: ["setTimeout"] });
		try {
			const { saver, calls } = setup([{ status: 200, etag: '"v2"' }], {
				id: "01OLD",
				etag: '"v1"',
			});
			saver.update(draft({ bodyMd: "W" }));
			mock.timers.tick(500);
			saver.update(draft({ bodyMd: "We" }));
			mock.timers.tick(999);
			assert.equal(calls.length, 0);
			mock.timers.tick(1);
			await saver.idle();
			assert.equal(calls.length, 1);
			assert.equal(calls[0].body.bodyMd, "We");
		} finally {
			mock.timers.reset();
		}
	});

	test("nothing is sent when nothing has changed since the last save", async () => {
		const { saver, calls } = setup([{ status: 200, etag: '"v2"' }], {
			id: "01OLD",
			etag: '"v1"',
		});
		saver.update(draft());
		await saver.flush();
		saver.update(draft());
		await saver.flush();
		assert.equal(calls.length, 1);
	});

	test("a new post without a headline isn't created yet", async () => {
		const { saver, calls, last } = setup([{ status: 201 }]);
		saver.update(draft({ title: "" }));
		await saver.flush();
		assert.equal(calls.length, 0);
		assert.deepEqual(last(), {
			kind: "invalid",
			fields: { title: "Add a headline" },
		});
	});

	test("someone else's save stops autosave with a clear message", async () => {
		const { saver, calls, last } = setup([{ status: 409 }], {
			id: "01OLD",
			etag: '"v1"',
		});
		saver.update(draft());
		await saver.flush();
		assert.equal(last()?.kind, "conflict");
		saver.update(draft({ bodyMd: "More." }));
		await saver.flush();
		assert.equal(
			calls.length,
			1,
			"kept saving over the other person's changes",
		);
	});

	test("fields the API rejects are shown against those fields", async () => {
		const { saver, last } = setup(
			[{ status: 422, json: { fields: { bodyMd: "The post is too long" } } }],
			{ id: "01OLD", etag: '"v1"' },
		);
		saver.update(draft());
		await saver.flush();
		assert.deepEqual(last(), {
			kind: "invalid",
			fields: { bodyMd: "The post is too long" },
		});
	});

	test("a failed save says so and is tried again on the next change", async () => {
		const { saver, calls, last } = setup(
			[{ status: 0 }, { status: 200, etag: '"v2"' }],
			{ id: "01OLD", etag: '"v1"' },
		);
		saver.update(draft());
		await saver.flush();
		assert.equal(last()?.kind, "failed");
		await saver.flush(); // same draft, but it never reached the API
		assert.equal(calls.length, 2);
		assert.equal(last()?.kind, "saved");
	});

	test("changes made while a save is under way are saved after it", async () => {
		const { saver, calls } = setup([
			{ status: 201, etag: '"v1"', json: { id: "01NEW" } },
			{ status: 200, etag: '"v2"' },
		]);
		saver.update(draft());
		const first = saver.flush();
		saver.update(draft({ bodyMd: "Typed while saving." }));
		await first;
		await saver.flush();
		assert.deepEqual(
			calls.map((c) => c.method),
			["POST", "PUT"],
			"created the post twice",
		);
		assert.equal(calls[1].body.bodyMd, "Typed while saving.");
	});

	test("it knows when there are changes that haven't been saved", async () => {
		const { saver } = setup([{ status: 0 }, { status: 200, etag: '"v2"' }], {
			id: "01OLD",
			etag: '"v1"',
		});
		assert.equal(saver.hasUnsavedChanges(), false);
		saver.update(draft());
		assert.equal(saver.hasUnsavedChanges(), true);
		await saver.flush(); // fails
		assert.equal(saver.hasUnsavedChanges(), true);
		await saver.flush();
		assert.equal(saver.hasUnsavedChanges(), false);
	});
});
