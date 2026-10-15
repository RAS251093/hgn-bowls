// @ts-check

import cloudflare from "@astrojs/cloudflare";
import mdx from "@astrojs/mdx";
import sitemap from "@astrojs/sitemap";
import { defineConfig } from "astro/config";
import markdownIntegration from "@astropub/md";
import tailwindcss from "@tailwindcss/vite";

// https://astro.build/config
export default defineConfig({
	vite: {
		plugins: [tailwindcss()],
	},
	site: "https://example.com",
	integrations: [
		mdx(),
		sitemap({ filter: (page) => !page.includes("/design-system") }),
		markdownIntegration(),
	],
	adapter: cloudflare({
		imageService: "compile",
	}),
	markdown: {
		remarkPlugins: [],
		rehypePlugins: [],
	},
});
