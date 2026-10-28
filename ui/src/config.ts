// Settings every page shares.

/** Where the Go API lives. Set PUBLIC_API_URL to point the site elsewhere (tests use a stub). */
export const apiUrl: string =
	import.meta.env.PUBLIC_API_URL ?? "https://api-e7vjhhfcma-uc.a.run.app";

/** The club's public Google Calendar, which the committee keeps up to date. */
export const calendarId =
	"4b13eeb3eb7dc7777ac193144fe1192c5ac182e82d018a9e35cf94d4788b5e3e@group.calendar.google.com";

const id = encodeURIComponent(calendarId);

export const calendarLinks = {
	/** Subscribes the visitor's phone or computer calendar to the club's. */
	subscribe: `webcal://calendar.google.com/calendar/ical/${id}/public/basic.ics`,
	/** The calendar itself, viewable without a Google account. */
	open: `https://calendar.google.com/calendar/embed?src=${id}&ctz=Europe%2FLondon`,
};
