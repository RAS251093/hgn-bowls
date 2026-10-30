import { expect, test } from "@playwright/test";

// Every public page. Add new pages here as they're built.
const pages = ["/", "/whats-on", "/news", "/news/sample-opening-day"];

// Linked from the header or footer but not built yet. Once one exists its
// check fails, as a reminder to take it off this list.
const notBuiltYet: Record<string, string> = {};

test.beforeEach(({ page }) =>
	page.route("https://api.test/api/events*", (route) =>
		route.fulfill({ json: { events: [], stale: false } }),
	),
);

for (const path of pages) {
	test.describe(path, () => {
		test("renders with one h1, the landmarks and no console errors", async ({
			page,
		}) => {
			const errors: string[] = [];
			page.on(
				"console",
				(msg) => msg.type() === "error" && errors.push(msg.text()),
			);
			page.on("pageerror", (err) => errors.push(err.message));

			const res = await page.goto(path);
			expect(res?.status()).toBe(200);
			await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
			await expect(page.getByRole("banner")).toBeVisible();
			await expect(page.getByRole("main")).toBeVisible();
			await expect(page.getByRole("contentinfo")).toBeVisible();
			await expect(page).toHaveTitle(/Hough Green Park Bowls Club/);
			await page.waitForLoadState("networkidle");
			expect(errors).toEqual([]);
		});

		test("every link to another page on the site works", async ({
			page,
			request,
		}) => {
			await page.goto(path);
			const hrefs = await page
				.locator("a[href^='/']")
				.evaluateAll((links) =>
					links.map((a) => (a as HTMLAnchorElement).pathname),
				);
			for (const href of new Set(hrefs)) {
				const res = await request.get(href);
				const ticket = notBuiltYet[href];
				if (ticket) {
					expect(
						res.status(),
						`${href} exists now: remove it from notBuiltYet (${ticket})`,
					).toBe(404);
				} else {
					expect(res.status(), `${path} links to ${href}`).toBe(200);
				}
			}
		});
	});
}

// The design system page is a tool for building the site (npm run dev),
// not something visitors should find.
test("the design system page isn't part of the built site", async ({
	request,
}) => {
	expect((await request.get("/design-system")).status()).toBe(404);
});

// Without a 404 page, Cloudflare Pages answers every unknown address with
// the home page and a 200, as if the site were a single-page app.
test("an unknown address gets a 404 page that leads back into the site", async ({
	page,
}) => {
	await page.route("https://api.test/api/events*", (route) =>
		route.fulfill({ json: { events: [], stale: false } }),
	);
	const res = await page.goto("/no-such-page");
	expect(res?.status()).toBe(404);
	await expect(page).toHaveTitle(/Page not found/);
	await expect(
		page.getByRole("heading", { level: 1, name: "Page not found" }),
	).toBeVisible();
	await expect(
		page.getByRole("main").getByRole("link", { name: "Go to the home page" }),
	).toHaveAttribute("href", "/");
	await expect(page.locator('meta[name="robots"]')).toHaveAttribute(
		"content",
		"noindex",
	);
});
