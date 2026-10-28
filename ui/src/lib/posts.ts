// Published posts for the site build, from GET /api/posts. Local and CI
// builds without a build token use the sample posts in src/fixtures; a
// Cloudflare Pages build without one fails, so samples never go live.
import { samplePosts } from "../fixtures/posts.ts";

export interface Post {
	id: string;
	slug: string;
	title: string;
	description: string;
	bodyMd: string;
	/** A one-hour signed link to the cover, or null for the club's default photo. */
	cover: { url: string; alt: string } | null;
	publishedAt: string;
	updatedAt: string;
}

interface ApiPost extends Omit<Post, "cover"> {
	cover: { key: string; url: string; alt: string } | null;
}

export interface LoadOptions {
	apiUrl: string;
	buildToken: string;
	/** True on Cloudflare Pages (CF_PAGES=1), where samples must never be used. */
	onPages: boolean;
	fetch?: typeof fetch;
}

export async function loadPosts({
	apiUrl,
	buildToken,
	onPages,
	fetch: get = fetch,
}: LoadOptions): Promise<Post[]> {
	if (!buildToken) {
		if (onPages) {
			const branch = process.env.CF_PAGES_BRANCH ?? "unknown";
			throw new Error(
				`posts: BUILD_TOKEN isn't set for this build (branch ${branch}), so it can't read posts. ` +
					"Add it under Settings, Variables and Secrets for both Production and Preview, then retry the deployment.",
			);
		}
		return newestFirst(samplePosts);
	}
	const res = await get(`${apiUrl}/api/posts`, {
		headers: { Authorization: `Bearer ${buildToken}` },
	});
	if (!res.ok) {
		throw new Error(`posts: GET /api/posts answered ${res.status}`);
	}
	const { posts } = (await res.json()) as { posts: ApiPost[] };
	return newestFirst(
		posts.map((p) => ({
			...p,
			cover: p.cover ? { url: p.cover.url, alt: p.cover.alt } : null,
		})),
	);
}

export function newestFirst(posts: Post[]): Post[] {
	return [...posts].sort(
		(a, b) => Date.parse(b.publishedAt) - Date.parse(a.publishedAt),
	);
}

/** "Friday 3 April", with the year added outside the current one. */
export function postDate(iso: string, now: Date = new Date()): string {
	const date = new Date(iso);
	const fmt = (o: Intl.DateTimeFormatOptions) =>
		new Intl.DateTimeFormat("en-GB", {
			timeZone: "Europe/London",
			...o,
		}).format(date);
	const thisYear = new Intl.DateTimeFormat("en-GB", {
		timeZone: "Europe/London",
		year: "numeric",
	}).format(now);
	const day = fmt({ weekday: "long", day: "numeric", month: "long" });
	const year = fmt({ year: "numeric" });
	return year === thisYear ? day : `${day} ${year}`;
}
