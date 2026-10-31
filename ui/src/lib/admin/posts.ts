// Posts as the admin pages see them (GET /api/admin/posts).

export interface AdminPost {
	id: string;
	title: string;
	status: "draft" | "published";
	/** Kept when a post is unpublished, so a draft may have one. */
	publishedAt: string | null;
	updatedAt: string;
	coverImageKey?: string | null;
	/** A link to the cover valid for an hour, when the post has one. */
	coverUrl?: string;
}

const london = (o: Intl.DateTimeFormatOptions) =>
	new Intl.DateTimeFormat("en-GB", { timeZone: "Europe/London", ...o });

const dayOf = (d: Date) =>
	london({ year: "numeric", month: "2-digit", day: "2-digit" }).format(d);

// "18 April", with the year added outside the current one.
function day(d: Date, now: Date): string {
	const sameYear =
		london({ year: "numeric" }).format(d) ===
		london({ year: "numeric" }).format(now);
	return london({
		day: "numeric",
		month: "long",
		...(sameYear ? {} : { year: "numeric" }),
	}).format(d);
}

/**
 * The line under a post's title: "Published 18 April" for a live post,
 * "Last edited 14:32" for a draft changed today (24-hour clock, as the
 * admin area uses), or "Last edited 29 October" before that.
 */
export function metaLine(post: AdminPost, now: Date = new Date()): string {
	if (post.status === "published" && post.publishedAt) {
		return `Published ${day(new Date(post.publishedAt), now)}`;
	}
	return `Last edited ${dayOrTime(new Date(post.updatedAt), now)}`;
}

/**
 * When something happened, in London: the time on the 24-hour clock if
 * it was today ("14:32"), otherwise the day ("29 October").
 */
export function dayOrTime(d: Date, now: Date = new Date()): string {
	if (dayOf(d) === dayOf(now)) {
		return london({
			hour: "2-digit",
			minute: "2-digit",
			hourCycle: "h23",
		}).format(d);
	}
	return day(d, now);
}

// The admin area is one page, so moving between the list, the editor and
// the help never reloads it (a reload means a trip through Auth0, as tokens
// live in memory). ?post=<id> opens the editor for that post, ?post=new a
// new one, ?help the guides (?help=posts, ?help=events or ?help=admins
// for one) and ?admins the people who can sign in.
export type Guide = "posts" | "events" | "admins";
export type Route =
	| { view: "list" }
	| { view: "edit"; id: string | null }
	| { view: "help"; guide: Guide | null }
	| { view: "admins" };

const guides: readonly string[] = ["posts", "events", "admins"];

/** The admins: who can sign in, and inviting more. */
export function adminsHref(): string {
	return "/admin?admins";
}

/** The guides, or one of them. */
export function helpHref(guide?: Guide): string {
	return guide ? `/admin?help=${guide}` : "/admin?help";
}

/** The editor for a post; without an id, a new one. */
export function editHref(id?: string): string {
	return `/admin?post=${id ? encodeURIComponent(id) : "new"}`;
}

/** Which view a URL's query string asks for. */
export function routeOf(search: string): Route {
	const params = new URLSearchParams(search);
	if (params.has("help")) {
		const guide = params.get("help") ?? "";
		return {
			view: "help",
			guide: guides.includes(guide) ? (guide as Guide) : null,
		};
	}
	if (params.has("admins")) return { view: "admins" };
	const post = params.get("post");
	if (!post) return { view: "list" };
	return { view: "edit", id: post === "new" ? null : post };
}
