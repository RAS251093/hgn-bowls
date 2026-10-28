import { defineConfig, devices } from "@playwright/test";

// Browser tests run against the built site. The API is never called for
// real: each test stubs https://api.test, which the build is pointed at.
export default defineConfig({
	testDir: "e2e",
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	reporter: process.env.CI ? "github" : "list",
	use: {
		baseURL: "http://localhost:4322",
	},
	projects: [
		{ name: "desktop", use: { ...devices["Desktop Chrome"] } },
		{ name: "phone", use: { ...devices["Pixel 7"] } },
	],
	webServer: {
		command: "npm run build && npm run preview -- --port 4322",
		url: "http://localhost:4322",
		env: { PUBLIC_API_URL: "https://api.test" },
		reuseExistingServer: !process.env.CI,
		timeout: 120_000,
	},
});
