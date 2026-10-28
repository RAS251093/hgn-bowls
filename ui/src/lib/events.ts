// Events from the club's Google Calendar, as GET /api/events returns them,
// and how the site writes their dates and times. Every date is shown in UK
// time, whatever offset the API sent it with.

/** One event from GET /api/events. */
export interface ClubEvent {
	id: string;
	title: string;
	/** RFC 3339 time, or a YYYY-MM-DD date when allDay is true. */
	start: string;
	/** As start; for an all-day event, the day after it ends. May be empty. */
	end: string;
	allDay: boolean;
	location: string;
	/** Plain text; may contain line breaks. */
	description: string;
}

/** What GET /api/events answers. stale means Google was down and these come from the last snapshot. */
export interface EventsResponse {
	events: ClubEvent[];
	stale: boolean;
}

const zone = "Europe/London";

// An all-day date has no time, so read it at midday UTC: the same calendar
// day in the UK whatever the season.
function instant(value: string, allDay: boolean): Date {
	return new Date(allDay ? `${value}T12:00:00Z` : value);
}

function format(date: Date, options: Intl.DateTimeFormatOptions): string {
	return new Intl.DateTimeFormat("en-GB", {
		timeZone: zone,
		...options,
	}).format(date);
}

/** The green date tile: Sat / 17 / Apr. */
export function tile(event: ClubEvent): {
	weekday: string;
	day: string;
	month: string;
} {
	const date = instant(event.start, event.allDay);
	return {
		weekday: format(date, { weekday: "short" }),
		day: format(date, { day: "numeric" }),
		month: format(date, { month: "short" }),
	};
}

/** The date written out: Saturday 17 April. */
export function fullDate(event: ClubEvent): string {
	return format(instant(event.start, event.allDay), {
		weekday: "long",
		day: "numeric",
		month: "long",
	});
}

/** A time the way the public pages write it: 2pm, 9:30am. */
function clock(value: string): string {
	const parts = new Intl.DateTimeFormat("en-GB", {
		timeZone: zone,
		hour: "numeric",
		minute: "2-digit",
		hourCycle: "h23",
	}).formatToParts(new Date(value));
	const hour = Number(parts.find((p) => p.type === "hour")?.value);
	const minute = parts.find((p) => p.type === "minute")?.value ?? "00";
	const suffix = hour < 12 ? "am" : "pm";
	const h12 = hour % 12 || 12;
	return minute === "00" ? `${h12}${suffix}` : `${h12}:${minute}${suffix}`;
}

/** The time and place line: "2pm to 4pm · Main green", or "All day". */
export function when(event: ClubEvent): string {
	let time = "All day";
	if (!event.allDay) {
		const start = clock(event.start);
		const end = event.end ? clock(event.end) : "";
		time = end && end !== start ? `${start} to ${end}` : start;
	}
	return event.location ? `${time} · ${event.location}` : time;
}

export interface MonthGroup {
	/** "November", or "January 2027" outside the current year. */
	label: string;
	events: ClubEvent[];
}

/** Groups events (already in date order) by their month in the UK. */
export function groupByMonth(
	events: ClubEvent[],
	now: Date = new Date(),
): MonthGroup[] {
	const thisYear = format(now, { year: "numeric" });
	const groups: MonthGroup[] = [];
	for (const event of events) {
		const date = instant(event.start, event.allDay);
		const month = format(date, { month: "long" });
		const year = format(date, { year: "numeric" });
		const label = year === thisYear ? month : `${month} ${year}`;
		const last = groups.at(-1);
		if (last?.label === label) {
			last.events.push(event);
		} else {
			groups.push({ label, events: [event] });
		}
	}
	return groups;
}

/** Fetches upcoming events. Throws if the API can't be reached or answers with an error. */
export async function fetchEvents(
	apiUrl: string,
	limit: number,
	signal?: AbortSignal,
): Promise<EventsResponse> {
	const res = await fetch(`${apiUrl}/api/events?limit=${limit}`, { signal });
	if (!res.ok) {
		throw new Error(`events: the API answered ${res.status}`);
	}
	return (await res.json()) as EventsResponse;
}
