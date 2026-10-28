import assert from "node:assert/strict";
import { describe, test } from "node:test";
import {
	type ClubEvent,
	fullDate,
	groupByMonth,
	tile,
	when,
} from "./events.ts";

const timed = (start: string, end: string, location = ""): ClubEvent => ({
	id: start,
	title: "Event",
	start,
	end,
	allDay: false,
	location,
	description: "",
});

const allDay = (start: string, end: string): ClubEvent => ({
	...timed(start, end),
	allDay: true,
});

describe("tile", () => {
	test("reads the date in UK time", () => {
		assert.deepEqual(tile(timed("2027-04-17T13:00:00+01:00", "")), {
			weekday: "Sat",
			day: "17",
			month: "Apr",
		});
	});

	test("an event late on a summer evening stays on its UK day", () => {
		// 23:30 UK time on Friday 2 July is Friday 22:30 UTC: still Friday.
		assert.equal(tile(timed("2027-07-02T22:30:00Z", "")).weekday, "Fri");
		// 00:30 UK time on Saturday 3 July is Friday 23:30 UTC: Saturday in the UK.
		assert.equal(tile(timed("2027-07-02T23:30:00Z", "")).weekday, "Sat");
	});

	test("an all-day event keeps its calendar date", () => {
		assert.deepEqual(tile(allDay("2026-11-02", "2026-11-03")), {
			weekday: "Mon",
			day: "2",
			month: "Nov",
		});
	});
});

describe("fullDate", () => {
	test("is written out for screen readers", () => {
		assert.equal(
			fullDate(timed("2027-04-17T13:00:00+01:00", "")),
			"Saturday 17 April",
		);
	});
});

describe("when", () => {
	const tests: { name: string; event: ClubEvent; want: string }[] = [
		{
			name: "on the hour",
			event: timed("2027-04-17T14:00:00+01:00", "2027-04-17T16:00:00+01:00"),
			want: "2pm to 4pm",
		},
		{
			name: "with minutes",
			event: timed("2026-11-02T09:30:00Z", "2026-11-02T10:30:00Z"),
			want: "9:30am to 10:30am",
		},
		{
			name: "noon and midnight",
			event: timed("2027-04-17T12:00:00+01:00", "2027-04-18T00:00:00+01:00"),
			want: "12pm to 12am",
		},
		{
			name: "UTC input shown in summer time",
			event: timed("2027-06-05T13:00:00Z", "2027-06-05T15:00:00Z"),
			want: "2pm to 4pm",
		},
		{
			name: "with a place",
			event: timed(
				"2027-04-17T14:00:00+01:00",
				"2027-04-17T16:00:00+01:00",
				"Main green",
			),
			want: "2pm to 4pm · Main green",
		},
		{
			name: "no end time",
			event: timed("2027-04-17T14:00:00+01:00", ""),
			want: "2pm",
		},
		{
			name: "start and end the same",
			event: timed("2027-04-17T14:00:00+01:00", "2027-04-17T14:00:00+01:00"),
			want: "2pm",
		},
		{
			name: "all day",
			event: allDay("2026-11-02", "2026-11-03"),
			want: "All day",
		},
		{
			name: "all day with a place",
			event: { ...allDay("2026-11-02", "2026-11-03"), location: "Clubhouse" },
			want: "All day · Clubhouse",
		},
	];
	for (const t of tests) {
		test(t.name, () => assert.equal(when(t.event), t.want));
	}
});

describe("groupByMonth", () => {
	const now = new Date("2026-11-01T10:00:00Z");

	test("groups in order, adding the year only when it isn't this year", () => {
		const events = [
			timed("2026-11-02T09:30:00Z", ""),
			timed("2026-11-30T23:30:00Z", ""), // 30 Nov in the UK
			allDay("2026-12-25", "2026-12-26"),
			timed("2027-01-04T10:00:00Z", ""),
		];
		const groups = groupByMonth(events, now);
		assert.deepEqual(
			groups.map((g) => [g.label, g.events.length]),
			[
				["November", 2],
				["December", 1],
				["January 2027", 1],
			],
		);
	});

	test("no events, no groups", () => {
		assert.deepEqual(groupByMonth([], now), []);
	});
});
