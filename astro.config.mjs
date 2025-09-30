// @ts-check

import cloudflare from "@astrojs/cloudflare";
import mdx from "@astrojs/mdx";
import sitemap from "@astrojs/sitemap";
import { defineConfig } from "astro/config";
import markdownIntegration from '@astropub/md';

// https://astro.build/config
export default defineConfig({
	vite: {
		plugins: [],
	},
	site: "https://example.com",
	integrations: [mdx(), sitemap(), markdownIntegration()],
	adapter: cloudflare({
		imageService: "compile"
	}),
	markdown: {
		remarkPlugins: [],
		rehypePlugins: [],
	},
});
