// Published posts, read from the API once per build (GET /api/posts with the
// build token). Bodies are rendered to sanitised HTML here, so pages only
// ever output the safe version.
import { defineCollection } from "astro:content";
import { z } from "astro/zod";
import { apiUrl } from "./config";
import { renderPost } from "./lib/markdown";
import { loadPosts } from "./lib/posts";

const posts = defineCollection({
	loader: {
		name: "posts-api",
		async load({ store, parseData, logger }) {
			const posts = await loadPosts({
				apiUrl,
				buildToken: process.env.BUILD_TOKEN ?? "",
				onPages: process.env.CF_PAGES === "1",
			});
			if (!process.env.BUILD_TOKEN)
				logger.warn("no BUILD_TOKEN, so the build uses the sample posts");
			store.clear();
			for (const post of posts) {
				const data = await parseData({ id: post.slug, data: { ...post } });
				store.set({
					id: post.slug,
					data,
					rendered: { html: await renderPost(post.bodyMd) },
				});
			}
			logger.info(`${posts.length} published posts`);
		},
	},
	schema: z.object({
		id: z.string(),
		slug: z.string(),
		title: z.string(),
		description: z.string(),
		bodyMd: z.string(),
		cover: z.object({ url: z.url(), alt: z.string() }).nullable(),
		publishedAt: z.string(),
		updatedAt: z.string(),
	}),
});

export const collections = { posts };
