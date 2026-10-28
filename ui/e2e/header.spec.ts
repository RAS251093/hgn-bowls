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
	await expect(nav.getByRole("link", { name: "Join us" })).toBeVisible();

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
	await expect(nav.getByRole("link", { name: "Join us" })).toBeVisible();
});
