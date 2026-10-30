import { expect, test } from "@playwright/test";

test.beforeEach(({ page }) =>
	page.route("https://api.test/api/events*", (route) =>
		route.fulfill({ json: { events: [], stale: false } }),
	),
);

test("on a phone, the nav sits behind a menu button", async ({
	page,
	isMobile,
}) => {
	test.skip(!isMobile, "phone layout");
	await page.goto("/");
	const nav = page.getByRole("navigation", { name: "Main" });
	const menu = page.getByRole("button", { name: "Menu" });

	await expect(menu).toHaveAttribute("aria-expanded", "false");
	await expect(nav.getByRole("link", { name: "What's on" })).toBeHidden();

	await menu.click();
	await expect(menu).toHaveAttribute("aria-expanded", "true");
	await expect(nav.getByRole("link", { name: "What's on" })).toBeVisible();

	await page.keyboard.press("Escape");
	await expect(menu).toHaveAttribute("aria-expanded", "false");
	await expect(menu).toBeFocused();
	await expect(nav.getByRole("link", { name: "What's on" })).toBeHidden();
});

test("on a phone, the header is one row", async ({ page, isMobile }) => {
	test.skip(!isMobile, "phone layout");
	await page.goto("/");
	const header = await page.getByRole("banner").boundingBox();
	expect(header?.height).toBeLessThanOrEqual(80);
});

test("on a desktop, the nav is always shown", async ({ page, isMobile }) => {
	test.skip(isMobile, "desktop layout");
	await page.goto("/whats-on");
	const nav = page.getByRole("navigation", { name: "Main" });
	await expect(page.getByRole("button", { name: "Menu" })).toBeHidden();
	await expect(nav.getByRole("link", { name: "What's on" })).toHaveAttribute(
		"aria-current",
		"page",
	);
	await expect(nav.getByRole("link")).toHaveText([
		"Home",
		"What's on",
		"News",
		"Find us",
	]);
});

// The club has no online sign-up yet, so nothing should promise one.
test("no page offers to join online", async ({ page }) => {
	for (const path of ["/", "/whats-on"]) {
		await page.goto(path);
		await expect(page.getByRole("link", { name: /join/i })).toHaveCount(0);
		await expect(page.locator('a[href^="/join"]')).toHaveCount(0);
	}
});

// Admins are sent the address; the public site doesn't advertise it.
test("no public page links to the admin area", async ({ page }) => {
	for (const path of ["/", "/whats-on", "/news", "/news/sample-opening-day"]) {
		await page.goto(path);
		await expect(page.locator('a[href^="/admin"]')).toHaveCount(0);
	}
});
