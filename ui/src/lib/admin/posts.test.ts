import assert from "node:assert/strict";
import { describe, test } from "node:test";
import {
	type AdminPost,
	adminsHref,
	dayOrTime,
	editHref,
	helpHref,
	metaLine,
	routeOf,
} from "./posts.ts";

const now = new Date("2026-10-30T15:00:00Z"); // 4pm in London (GMT)

const post = (p: Partial<AdminPost>): AdminPost => ({
	id: "01ABC",
	title: "Opening day",
	status: "draft",
	publishedAt: null,
	updatedAt: "2026-10-30T14:32:00.000000Z",
	...p,
});

describe("metaLine", () => {
	test("a published post shows its publish date, with the year when it isn't this one", () => {
		assert.equal(
			metaLine(
				post({
					status: "published",
					publishedAt: "2026-04-18T09:00:00.000000Z",
				}),
				now,
			),
			"Published 18 April",
		);
		assert.equal(
			metaLine(
				post({
					status: "published",
					publishedAt: "2025-02-01T00:00:00.000000Z",
				}),
				now,
			),
			"Published 1 February 2025",
		);
	});

	test("a draft edited today shows the time, on the 24-hour clock", () => {
		assert.equal(metaLine(post({}), now), "Last edited 14:32");
		// UK time: 23:30 UTC in summer is already the next day in London.
		assert.equal(
			metaLine(
				post({ updatedAt: "2026-07-14T23:30:00.000000Z" }),
				new Date("2026-07-15T09:00:00Z"),
			),
			"Last edited 00:30",
		);
	});

	test("a draft edited before today shows the day", () => {
		assert.equal(
			metaLine(post({ updatedAt: "2026-10-29T18:05:00.000000Z" }), now),
			"Last edited 29 October",
		);
		assert.equal(
			metaLine(post({ updatedAt: "2025-12-24T10:00:00.000000Z" }), now),
			"Last edited 24 December 2025",
		);
	});

	test("an unpublished post that was once live reads as a draft", () => {
		assert.equal(
			metaLine(
				post({ status: "draft", publishedAt: "2026-04-18T09:00:00.000000Z" }),
				now,
			),
			"Last edited 14:32",
		);
	});
});

describe("dayOrTime", () => {
	test("today is the time on the 24-hour clock, in London", () => {
		assert.equal(dayOrTime(new Date("2026-10-30T08:05:00Z"), now), "08:05");
		assert.equal(
			dayOrTime(
				new Date("2026-07-14T23:30:00Z"),
				new Date("2026-07-15T09:00:00Z"),
			),
			"00:30",
		);
	});

	test("before today is the day, with the year when it isn't this one", () => {
		assert.equal(
			dayOrTime(new Date("2026-10-29T23:59:00Z"), now),
			"29 October",
		);
		assert.equal(
			dayOrTime(new Date("2025-03-02T12:00:00Z"), now),
			"2 March 2025",
		);
	});
});

describe("editHref", () => {
	test("links to the editor for that post, on the one admin page", () => {
		assert.equal(editHref("01ABC"), "/admin?post=01ABC");
	});

	test("without an id, a new post", () => {
		assert.equal(editHref(), "/admin?post=new");
	});

	test("escapes the id", () => {
		assert.equal(editHref("a&b"), "/admin?post=a%26b");
	});
});

describe("routeOf", () => {
	test("the admin page shows the list, or the editor for ?post=", () => {
		assert.deepEqual(routeOf(""), { view: "list" });
		assert.deepEqual(routeOf("?post=01ABC"), { view: "edit", id: "01ABC" });
		assert.deepEqual(routeOf("?post=new"), { view: "edit", id: null });
		assert.deepEqual(routeOf("?post="), { view: "list" });
	});

	test("reads its own links back", () => {
		assert.deepEqual(routeOf(new URL(editHref("a&b"), "https://x").search), {
			view: "edit",
			id: "a&b",
		});
	});
});

describe("admins", () => {
	test("?admins opens the admins", () => {
		assert.deepEqual(routeOf("?admins"), { view: "admins" });
		assert.deepEqual(routeOf("?admins="), { view: "admins" });
	});

	test("the link reads back as the admins", () => {
		assert.equal(adminsHref(), "/admin?admins");
		assert.deepEqual(routeOf(new URL(adminsHref(), "https://x").search), {
			view: "admins",
		});
	});
});

describe("help", () => {
	test("?help opens the guides, ?help=<guide> one of them", () => {
		assert.deepEqual(routeOf("?help"), { view: "help", guide: null });
		assert.deepEqual(routeOf("?help=events"), {
			view: "help",
			guide: "events",
		});
		assert.deepEqual(routeOf("?help=posts"), { view: "help", guide: "posts" });
		assert.deepEqual(routeOf("?help=admins"), {
			view: "help",
			guide: "admins",
		});
		assert.deepEqual(routeOf("?help=nonsense"), { view: "help", guide: null });
	});

	test("links read back as the same guide", () => {
		assert.equal(helpHref(), "/admin?help");
		assert.equal(helpHref("events"), "/admin?help=events");
		assert.deepEqual(routeOf(new URL(helpHref("posts"), "https://x").search), {
			view: "help",
			guide: "posts",
		});
	});
});
