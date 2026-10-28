// @ts-check
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";

// https://astro.build/config
export default defineConfig({
	site: "https://houghgreenparkbowls.uk",
	// Plain HTML from Cloudflare Pages: nothing renders on a server per visit.
	output: "static",
	vite: {
		plugins: [tailwindcss()],
	},
});
