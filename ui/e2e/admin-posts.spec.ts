import { expect, type Page, test } from "@playwright/test";
import { stubAuth0, stubPosts } from "./support/admin";

// The admin home is the list of news posts, read from GET /api/admin/posts
// (most recently changed first) once the admin is signed in.

const posts = [
	{
		id: "01DRAFT",
		title: "Winter league fixtures",
		status: "draft",
		publishedAt: null,
		updatedAt: "2026-10-29T18:05:00.000000Z",
	},
	{
		id: "01LIVE",
		title: "Opening day on the green",
		status: "published",
		publishedAt: "2025-04-05T09:00:00.000000Z",
		updatedAt: "2025-04-05T09:00:00.000000Z",
	},
];

// Answers GET /api/admin/posts/<id> with that post from the list.
async function stubPost(page: Page) {
	await page.route("https://api.test/api/admin/posts/*", (route) => {
		const id = new URL(route.request().url()).pathname.split("/").pop();
		const post = posts.find((p) => p.id === id);
		return route.fulfill({
			headers: {
				"Access-Control-Allow-Origin": "*",
				"Access-Control-Expose-Headers": "ETag",
				ETag: '"v1"',
			},
			json: {
				...post,
				description: "",
				bodyMd: "",
				coverImageKey: null,
				coverAlt: null,
			},
		});
	});
}

async function openList(page: Page, list: object[] = posts) {
	await stubAuth0(page);
	await stubPosts(page, [{ status: 200, json: { posts: list } }]);
	await page.goto("/admin");
	await expect(page.getByRole("link", { name: "Write a post" })).toBeVisible();
}

test("every post is listed with its status in words and when it last changed", async ({
	page,
}) => {
	await openList(page);
	const rows = page.getByRole("list", { name: "Posts" }).getByRole("listitem");
	await expect(rows).toHaveCount(2);

	await expect(rows.nth(0)).toContainText("Winter league fixtures");
	await expect(rows.nth(0)).toContainText("Draft");
	await expect(rows.nth(0)).toContainText("Last edited 29 October");

	await expect(rows.nth(1)).toContainText("Opening day on the green");
	await expect(rows.nth(1)).toContainText("Published");
	await expect(rows.nth(1)).toContainText("Published 5 April 2025");
});

test("each row is one link to edit that post, named by its title", async ({
	page,
}) => {
	await openList(page);
	const rows = page.getByRole("list", { name: "Posts" }).getByRole("listitem");
	await expect(rows.nth(0).getByRole("link")).toHaveCount(1);
	await expect(
		page.getByRole("link", { name: "Winter league fixtures" }),
	).toHaveAttribute("href", "/admin?post=01DRAFT");
	// Deleting happens in the editor, never one slip away on the list.
	await expect(page.getByRole("button", { name: /delete/i })).toHaveCount(0);
});

test("the whole row opens the editor, not just the title", async ({ page }) => {
	await openList(page);
	await stubPost(page);
	const row = page
		.getByRole("list", { name: "Posts" })
		.getByRole("listitem")
		.nth(0);
	const box = await row.boundingBox();
	if (!box) throw new Error("row has no box");
	// Click near the right-hand end, well away from the title text.
	await page.mouse.click(box.x + box.width - 20, box.y + box.height / 2);
	await expect(page).toHaveURL(/\/admin\?post=01DRAFT$/);
	await expect(
		page.getByRole("heading", { level: 1, name: "Edit post" }),
	).toBeVisible();
});

test("Write a post is the page's main action", async ({ page }) => {
	await openList(page);
	await expect(
		page.getByRole("link", { name: "Write a post" }),
	).toHaveAttribute("href", "/admin?post=new");
});

test("events are pointed to Google Calendar", async ({ page }) => {
	await openList(page);
	await expect(
		page.getByRole("link", { name: "Open Google Calendar" }),
	).toHaveAttribute("href", /^https:\/\/calendar\.google\.com\//);
});

test("with no posts yet, the page says so and leads to writing one", async ({
	page,
}) => {
	await openList(page, []);
	await expect(page.getByRole("list", { name: "Posts" })).toBeHidden();
	await expect(page.getByText("No news posts yet.")).toBeVisible();
	await expect(page.getByRole("link", { name: "Write a post" })).toBeVisible();
});

test("titles are shown as text, never as HTML", async ({ page }) => {
	await openList(page, [
		{ ...posts[0], title: '<img src=x onerror="window.hacked=1">' },
	]);
	await expect(
		page.getByRole("link", { name: '<img src=x onerror="window.hacked=1">' }),
	).toBeVisible();
	expect(await page.evaluate(() => "hacked" in window)).toBe(false);
});

test("the list works from the keyboard alone", async ({ page }) => {
	await openList(page);
	// Tab through the page, noting what takes focus, until the second row.
	const order: string[] = [];
	for (let i = 0; i < 10 && order.at(-1) !== "Opening day on the green"; i++) {
		await page.keyboard.press("Tab");
		order.push(
			await page.evaluate(
				() => document.activeElement?.textContent?.trim() ?? "",
			),
		);
	}
	const write = order.indexOf("Write a post");
	const first = order.indexOf("Winter league fixtures");
	expect(write).toBeGreaterThanOrEqual(0);
	expect(first).toBeGreaterThan(write);
	expect(order.at(-1)).toBe("Opening day on the green");
	expect(order.length).toBe(first + 2);

	await stubPost(page);
	await page.keyboard.press("Enter");
	await expect(page).toHaveURL(/\/admin\?post=01LIVE$/);
	// Focus moves to the editor's heading, so a screen reader announces it.
	await expect(
		page.getByRole("heading", { level: 1, name: "Edit post" }),
	).toBeFocused();
});

test("a list that won't load says what to do", async ({ page }) => {
	await stubAuth0(page);
	await stubPosts(page, [{ status: 500 }]);
	await page.goto("/admin");
	await expect(page.getByRole("alert")).toContainText(
		"couldn't load the posts",
	);
});
