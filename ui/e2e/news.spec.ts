import { expect, test } from "@playwright/test";

// These run against the sample posts in src/fixtures/posts.ts, which the
// build uses when it has no BUILD_TOKEN.
test.beforeEach(({ page }) =>
	page.route("https://api.test/api/events*", (route) =>
		route.fulfill({ json: { events: [], stale: false } }),
	),
);

test("the news page lists every post, newest first", async ({ page }) => {
	await page.goto("/news");
	await expect(
		page.getByRole("heading", { level: 1, name: "Club news" }),
	).toBeVisible();
	await expect(page.getByRole("heading", { level: 2 })).toHaveText([
		"Sample post: opening day on the green",
		"Sample post: coaching sessions for beginners",
		"Sample post: end of season social",
		"Sample post: green maintenance this winter",
	]);
	await expect(
		page.getByRole("link", { name: "Sample post: opening day on the green" }),
	).toHaveAttribute("href", "/news/sample-opening-day");
	// On phones the nav sits behind the menu, so check the link itself.
	await expect(
		page.locator('nav[aria-label="Main"] a[href="/news"]'),
	).toHaveAttribute("aria-current", "page");
});

test("dates are written out, with the year for older posts", async ({
	page,
}) => {
	await page.goto("/news");
	await expect(page.locator("time").first()).toHaveText(
		/^\w+day \d{1,2} \w+( \d{4})?$/,
	);
	await expect(page.locator("time[datetime^='2025-09-30']")).toHaveText(
		"Tuesday 30 September 2025",
	);
});

test("a post shows its title, date, body and more news", async ({ page }) => {
	await page.goto("/news/sample-opening-day");
	await expect(page.getByRole("heading", { level: 1 })).toHaveText(
		"Sample post: opening day on the green",
	);
	await expect(page.locator("main > article time")).toHaveAttribute(
		"datetime",
		"2026-10-20T10:00:00.000000Z",
	);
	await expect(
		page.getByRole("heading", { level: 2, name: "How the day went" }),
	).toBeVisible();
	await expect(page.locator("main > article strong")).toHaveText("sample post");
	await expect(
		page.getByRole("link", { name: "All club news" }),
	).toHaveAttribute("href", "/news");

	const more = page.getByRole("region", { name: "More club news" });
	await expect(more.getByRole("article")).toHaveCount(3);
	await expect(more).not.toContainText("opening day on the green");
});

test("a post without a cover shows the club photo, as decoration", async ({
	page,
}) => {
	await page.goto("/news/sample-opening-day");
	const cover = page.locator("main > article picture img");
	await expect(cover).toBeVisible();
	await expect(cover).toHaveAttribute("alt", "");
	await expect(cover).toHaveAttribute("src", /two-crown-green-bowls/);
});

// Covers are cropped to their frame and resized at build. Real covers are
// remote and big (a 3130x1565 photo), so every size here is set
// explicitly, never taken from the original.
test("covers are cropped and resized to their frame", async ({ page }) => {
	await page.goto("/news/sample-opening-day");
	const cover = page.locator("main > article picture img");
	await expect(cover).toHaveAttribute("width", "1000");
	await expect(cover).toHaveAttribute("height", "563");
	await expect(cover).toHaveAttribute("src", /\.jpg$/);

	await page.goto("/news");
	const card = page.locator("article img").first();
	await expect(card).toHaveAttribute("width", "720");
	await expect(card).toHaveAttribute("height", "480");
	const widths = (await card.getAttribute("srcset"))
		?.split(",")
		.map((s) => s.trim().split(" ")[1]);
	expect(widths).toEqual(["360w", "560w", "720w"]);
});

test("the home page shows the latest three posts", async ({ page }) => {
	await page.goto("/");
	const news = page.getByRole("region", { name: "Club news" });
	await expect(news.getByRole("article")).toHaveCount(3);
	await expect(news.getByRole("heading", { level: 3 }).first()).toHaveText(
		"Sample post: opening day on the green",
	);
	await expect(
		news.getByRole("link", { name: "All club news" }),
	).toHaveAttribute("href", "/news");
});

test("old addresses redirect to the new pages", async ({ request }) => {
	const rules = await (await request.get("/_redirects")).text();
	expect(rules).toContain("/blog/* /news/:splat 301");
	expect(rules).toContain("/blog /news 301");
	expect(rules).toContain("/events /whats-on 301");
});

test("a video loads YouTube only when it's played", async ({ page }) => {
	const youtube: string[] = [];
	page.on(
		"request",
		(req) => /youtube|ytimg/.test(req.url()) && youtube.push(req.url()),
	);
	await page.route(/youtube-nocookie\.com/, (route) =>
		route.fulfill({ body: "player" }),
	);

	await page.goto("/news/sample-coaching-sessions");
	const play = page.getByRole("link", { name: "Play video: Watch on YouTube" });
	await expect(play).toHaveAttribute(
		"href",
		"https://www.youtube.com/watch?v=LegryDpTHXo",
	);
	await page.waitForLoadState("networkidle");
	expect(youtube).toEqual([]);

	await play.click();
	const player = page.locator("figure[data-youtube] iframe");
	await expect(player).toHaveAttribute(
		"src",
		"https://www.youtube-nocookie.com/embed/LegryDpTHXo?autoplay=1&start=0",
	);
	await expect(play).toHaveCount(0);
});
