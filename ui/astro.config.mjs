// @ts-check
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";

// The design system page (every component, for building the site) is
// served by `astro dev` only, so it's never in a build visitors can reach.
/** @type {import("astro").AstroIntegration} */
const designSystemInDev = {
	name: "design-system-in-dev",
	hooks: {
		"astro:config:setup": ({ command, injectRoute }) => {
			if (command === "dev") {
				injectRoute({
					pattern: "/design-system",
					entrypoint: "./src/dev/design-system.astro",
				});
			}
		},
	},
};

// https://astro.build/config
export default defineConfig({
	site: "https://houghgreenparkbowls.uk",
	// Plain HTML from Cloudflare Pages: nothing renders on a server per visit.
	output: "static",
	integrations: [designSystemInDev],
	image: {
		// Post covers arrive as signed Cloud Storage links; the build downloads
		// and resizes them, so visitors never load them from the bucket.
		remotePatterns: [{ protocol: "https", hostname: "storage.googleapis.com" }],
	},
	vite: {
		plugins: [tailwindcss()],
	},
});
