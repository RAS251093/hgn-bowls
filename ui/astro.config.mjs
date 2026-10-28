// @ts-check
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";

// https://astro.build/config
export default defineConfig({
	site: "https://houghgreenparkbowls.uk",
	// Plain HTML from Cloudflare Pages: nothing renders on a server per visit.
	output: "static",
	image: {
		// Post covers arrive as signed Cloud Storage links; the build downloads
		// and resizes them, so visitors never load them from the bucket.
		remotePatterns: [{ protocol: "https", hostname: "storage.googleapis.com" }],
	},
	vite: {
		plugins: [tailwindcss()],
	},
});
