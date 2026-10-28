// Sample posts for local and CI builds, which have no build token. They
// exercise the page layouts and never reach the live site.
import type { Post } from "../lib/posts.ts";

export const samplePosts: Post[] = [
	{
		id: "sample-1",
		slug: "sample-opening-day",
		title: "Sample post: opening day on the green",
		description:
			"A sample post for building the site without the API. It never goes live.",
		bodyMd:
			"This is a **sample post**, used when the site is built without the API.\n\n## How the day went\n\nParagraphs, lists and [links](https://example.org) all render as they will for real posts.\n\n- Two friendly matches\n- Tea in the clubhouse",
		cover: null,
		publishedAt: "2026-10-20T10:00:00.000000Z",
		updatedAt: "2026-10-20T10:00:00.000000Z",
	},
	{
		id: "sample-2",
		slug: "sample-coaching-sessions",
		title: "Sample post: coaching sessions for beginners",
		description:
			"A second sample post with a longer description, to check how cards wrap when the text runs to two lines.",
		bodyMd: "Another **sample post** for local builds.",
		cover: null,
		publishedAt: "2026-09-12T10:00:00.000000Z",
		updatedAt: "2026-09-12T10:00:00.000000Z",
	},
	{
		id: "sample-3",
		slug: "sample-end-of-season",
		title: "Sample post: end of season social",
		description: "A third sample post.",
		bodyMd: "A third **sample post**.",
		cover: null,
		publishedAt: "2025-09-30T10:00:00.000000Z",
		updatedAt: "2025-09-30T10:00:00.000000Z",
	},
	{
		id: "sample-4",
		slug: "sample-green-maintenance",
		title: "Sample post: green maintenance this winter",
		description:
			"A fourth sample post, so the home page has more than it shows.",
		bodyMd: "A fourth **sample post**.",
		cover: null,
		publishedAt: "2025-08-01T10:00:00.000000Z",
		updatedAt: "2025-08-01T10:00:00.000000Z",
	},
];
