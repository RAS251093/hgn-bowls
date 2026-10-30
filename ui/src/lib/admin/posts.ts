// Posts as the admin pages see them (GET /api/admin/posts).

export interface AdminPost {
	status: "draft" | "published";
}

const count = (n: number, one: string, many: string) =>
	`${n} ${n === 1 ? one : many}`;

/** "2 published posts and 1 draft", for the admin home page. */
export function describePosts(posts: AdminPost[]): string {
	const published = posts.filter((p) => p.status === "published").length;
	const drafts = posts.length - published;
	const parts = [
		published && count(published, "published post", "published posts"),
		drafts && count(drafts, "draft", "drafts"),
	].filter(Boolean);
	return parts.length ? parts.join(" and ") : "No posts yet";
}
