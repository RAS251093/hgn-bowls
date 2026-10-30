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
	const edited = new Date(post.updatedAt);
	if (dayOf(edited) === dayOf(now)) {
		const time = london({
			hour: "2-digit",
			minute: "2-digit",
			hourCycle: "h23",
		}).format(edited);
		return `Last edited ${time}`;
	}
	return `Last edited ${day(edited, now)}`;
}

// The admin area is one page, so moving between the list and the editor
// never reloads it (a reload means a trip through Auth0, as tokens live in
// memory). ?post=<id> opens the editor for that post, ?post=new a new one.
export type Route = { view: "list" } | { view: "edit"; id: string | null };

/** The editor for a post; without an id, a new one. */
export function editHref(id?: string): string {
	return `/admin?post=${id ? encodeURIComponent(id) : "new"}`;
}

/** Which view a URL's query string asks for. */
export function routeOf(search: string): Route {
	const post = new URLSearchParams(search).get("post");
	if (!post) return { view: "list" };
	return { view: "edit", id: post === "new" ? null : post };
}
