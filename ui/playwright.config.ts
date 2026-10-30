import { defineConfig, devices } from "@playwright/test";

// Browser tests run against the built site. The API and Auth0 are never
// called for real: tests stub https://api.test and https://auth.test, which
// the build is pointed at.
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
		env: {
			PUBLIC_API_URL: "https://api.test",
			PUBLIC_AUTH0_DOMAIN: "auth.test",
			PUBLIC_AUTH0_CLIENT_ID: "test-client",
			PUBLIC_DEPLOY_POLL_MS: "100",
		},
		reuseExistingServer: !process.env.CI,
		timeout: 120_000,
	},
});
