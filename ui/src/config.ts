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

/** The club itself. Leave email or phone undefined to keep them off the site. */
export const club = {
	name: "Hough Green Park Bowls Club",
	street: "95 Mayfield Avenue",
	town: "Widnes WA8 8PW",
	facebook: "https://www.facebook.com/houghgreenparkclub/",
	email: undefined as string | undefined,
	phone: undefined as string | undefined,
};

const place = encodeURIComponent(
	"Hough Green Park Bowls Club, 95 Mayfield Ave, Widnes WA8 8PW",
);

/** Google Maps URLs (no API key): the club on the map, and directions to it from wherever the visitor is. */
export const mapLinks = {
	open: `https://www.google.com/maps/search/?api=1&query=${place}`,
	directions: `https://www.google.com/maps/dir/?api=1&destination=${place}`,
};

/** The Auth0 tenant the admin area signs in with. Tests point it at a stub. */
export const auth0 = {
	domain: import.meta.env.PUBLIC_AUTH0_DOMAIN ?? "hgn-bowls.uk.auth0.com",
	clientId:
		import.meta.env.PUBLIC_AUTH0_CLIENT_ID ??
		"ghYs1Bt8BnKD6mEA7HTCDSFVS8Lq9P9b",
	/** The API the admin's access tokens are for. */
	audience: "https://houghgreenparkbowls.uk/api",
};
