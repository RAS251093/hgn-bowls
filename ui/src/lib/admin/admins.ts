// The people who can sign in to the admin area (GET /api/admin/admins).
import { dayOrTime } from "./posts.ts";

export interface Admin {
	/** Their Auth0 user id. */
	id: string;
	name: string;
	email: string;
	/** Invited until they first sign in. */
	status: "invited" | "active";
	lastLogin: string | null;
}

/** The signed-in admin, to find their own row. */
export interface Me {
	sub?: string;
	email?: string;
}

/**
 * The line under an admin's name: "Invited, hasn't signed in yet", or
 * "Last signed in at 14:32" today (24-hour clock) and "Last signed in on
 * 29 October" before that.
 */
export function statusLine(admin: Admin, now: Date = new Date()): string {
	if (admin.status === "invited") return "Invited, hasn't signed in yet";
	if (!admin.lastLogin) return "Hasn't signed in yet";
	const when = dayOrTime(new Date(admin.lastLogin), now);
	// A time means today: "at 14:32"; otherwise a date: "on 29 October".
	return `Last signed in ${/^\d{2}:\d{2}$/.test(when) ? "at" : "on"} ${when}`;
}

/** Whether a row is the signed-in admin's own, by Auth0 id or email address. */
export function isYou(admin: Admin, me: Me): boolean {
	if (me.sub && admin.id === me.sub) return true;
	return !!me.email && admin.email.toLowerCase() === me.email.toLowerCase();
}

/** The admins in name order, as the API sends them, with one added or replaced. */
export function withAdmin(admins: readonly Admin[], admin: Admin): Admin[] {
	return [...admins.filter((a) => a.id !== admin.id), admin].sort((a, b) =>
		a.name.localeCompare(b.name, "en-GB", { sensitivity: "base" }),
	);
}
