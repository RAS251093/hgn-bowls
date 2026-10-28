import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
	await page.route("https://api.test/api/events*", (route) =>
		route.fulfill({ json: { events: [], stale: false } }),
	);
	await page.goto("/");
});

test("has one h1 and the three club facts", async ({ page }) => {
	await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
	for (const fact of [
		"All ages and abilities",
		"Qualified coaches",
		"A full programme",
	]) {
		await expect(
			page.getByRole("heading", { level: 2, name: fact }),
		).toBeVisible();
	}
});

test("the hero sends visitors to Find us and What's on", async ({ page }) => {
	const hero = page.getByRole("region", {
		name: "Bowls in the heart of Hough Green Park",
	});
	await expect(hero.getByRole("link", { name: "Find us" })).toHaveAttribute(
		"href",
		"/#find-us",
	);
	await expect(hero.getByRole("link", { name: "What's on" })).toHaveAttribute(
		"href",
		"/whats-on",
	);
});

test("Find us gives the address and opens Google Maps", async ({ page }) => {
	const findUs = page.getByRole("region", { name: "Find us" });
	await expect(findUs.locator("address")).toContainText("95 Mayfield Avenue");
	await expect(findUs.locator("address")).toContainText("Widnes WA8 8PW");

	const open = new URL(
		(await findUs
			.getByRole("link", { name: "Open in Google Maps" })
			.getAttribute("href")) ?? "",
	);
	expect(open.origin + open.pathname).toBe(
		"https://www.google.com/maps/search/",
	);
	expect(open.searchParams.get("api")).toBe("1");
	expect(open.searchParams.get("query")).toBe(
		"Hough Green Park Bowls Club, 95 Mayfield Ave, Widnes WA8 8PW",
	);

	const directions = new URL(
		(await findUs
			.getByRole("link", { name: "Get directions" })
			.getAttribute("href")) ?? "",
	);
	expect(directions.origin + directions.pathname).toBe(
		"https://www.google.com/maps/dir/",
	);
	expect(directions.searchParams.get("destination")).toBe(
		"Hough Green Park Bowls Club, 95 Mayfield Ave, Widnes WA8 8PW",
	);
});

test("the Find us nav link lands on the section", async ({ page }) => {
	await page.goto("/whats-on");
	await page.goto("/#find-us");
	await expect(page.getByRole("region", { name: "Find us" })).toBeInViewport();
});

test("the footer links to Facebook and hides contact details we don't have", async ({
	page,
}) => {
	const footer = page.getByRole("contentinfo");
	await expect(footer.getByRole("link", { name: "Facebook" })).toHaveAttribute(
		"href",
		"https://www.facebook.com/houghgreenparkclub/",
	);
	await expect(footer).not.toContainText("[Club email]");
	await expect(footer).not.toContainText("[Phone number]");
});
