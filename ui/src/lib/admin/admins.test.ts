import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { type Admin, isYou, statusLine, withAdmin } from "./admins.ts";

const now = new Date("2026-10-30T15:00:00Z"); // 3pm in London (GMT)

const admin = (a: Partial<Admin>): Admin => ({
	id: "auth0|ann",
	name: "Ann Jones",
	email: "ann@example.org",
	status: "active",
	lastLogin: null,
	...a,
});

describe("statusLine", () => {
	test("someone invited hasn't signed in yet", () => {
		assert.equal(
			statusLine(admin({ status: "invited" }), now),
			"Invited, hasn't signed in yet",
		);
	});

	test("today's sign-in shows the time, on the 24-hour clock in London", () => {
		assert.equal(
			statusLine(admin({ lastLogin: "2026-10-30T14:32:00Z" }), now),
			"Last signed in at 14:32",
		);
		// UK time: 23:30 UTC in summer is already the next day in London.
		assert.equal(
			statusLine(
				admin({ lastLogin: "2026-07-14T23:30:00Z" }),
				new Date("2026-07-15T09:00:00Z"),
			),
			"Last signed in at 00:30",
		);
	});

	test("an earlier sign-in shows the day, with the year when it isn't this one", () => {
		assert.equal(
			statusLine(admin({ lastLogin: "2026-10-29T18:05:00Z" }), now),
			"Last signed in on 29 October",
		);
		assert.equal(
			statusLine(admin({ lastLogin: "2025-12-24T10:00:00Z" }), now),
			"Last signed in on 24 December 2025",
		);
	});

	test("an active admin with no sign-in recorded says so", () => {
		assert.equal(statusLine(admin({}), now), "Hasn't signed in yet");
	});
});

describe("isYou", () => {
	test("matches the signed-in admin by Auth0 id", () => {
		assert.equal(isYou(admin({}), { sub: "auth0|ann" }), true);
		assert.equal(isYou(admin({}), { sub: "auth0|rob" }), false);
	});

	test("or by email address, ignoring case", () => {
		assert.equal(isYou(admin({}), { email: "Ann@Example.org" }), true);
		assert.equal(isYou(admin({}), { email: "rob@example.org" }), false);
	});

	test("knowing nothing about the signed-in admin matches nobody", () => {
		assert.equal(isYou(admin({}), {}), false);
		assert.equal(isYou(admin({ email: "" }), { email: "" }), false);
	});
});

describe("withAdmin", () => {
	const bob = admin({ id: "b", name: "bob Hale" });
	const cat = admin({ id: "c", name: "Cat Moss" });

	test("adds an admin in name order", () => {
		assert.deepEqual(
			withAdmin([bob, cat], admin({ id: "a", name: "Ann" })).map((a) => a.id),
			["a", "b", "c"],
		);
		assert.deepEqual(
			withAdmin([bob, cat], admin({ id: "d", name: "Dee" })).map((a) => a.id),
			["b", "c", "d"],
		);
	});

	test("replaces one already there", () => {
		const list = withAdmin([bob, cat], { ...cat, status: "invited" });
		assert.equal(list.length, 2);
		assert.equal(list[1].status, "invited");
	});
});
