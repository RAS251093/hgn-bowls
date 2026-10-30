import { expect, type Page, test } from "@playwright/test";
import { stubAuth0 } from "./support/admin";

const api = "https://api.test/api/admin/posts";
const cors = {
	"Access-Control-Allow-Origin": "*",
	"Access-Control-Expose-Headers": "ETag",
};
const png = Buffer.from(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=",
	"base64",
);

const stored = {
	id: "01LIVE",
	title: "Opening day",
	description: "The green opens for the season.",
	bodyMd:
		"We opened the green.\n\n## How it went\n\nIt was **sunny**.\n\n<script>window.hacked = 1</script>",
	coverImageKey: "posts/01COVER.jpg",
	coverAlt: "Two bowlers on the green",
	coverUrl: "https://storage.test/posts/01COVER.jpg?sig",
	status: "published",
	publishedAt: "2025-04-05T09:00:00.000000Z",
	updatedAt: "2025-04-05T09:00:00.000000Z",
};

/** Stubs the API for one post that can be deleted and restored. */
async function stubApi(page: Page, post: object = stored) {
	const calls: string[] = [];
	let deleted = false;
	await page.route("https://storage.test/**", (route) =>
		route.fulfill({ contentType: "image/png", body: png }),
	);
	await page.route(api, (route) =>
		route.fulfill({ json: { posts: deleted ? [] : [post] } }),
	);
	await page.route(`${api}/**`, async (route) => {
		const req = route.request();
		const path = new URL(req.url()).pathname;
		calls.push(`${req.method()} ${path}`);
		if (req.method() === "DELETE") {
			deleted = true;
			return route.fulfill({
				headers: cors,
				json: { rebuild: { status: "started" } },
			});
		}
		if (path.endsWith("/restore")) {
			deleted = false;
			return route.fulfill({
				headers: { ...cors, ETag: '"v3"' },
				json: { post, rebuild: { status: "started" } },
			});
		}
		if (req.method() === "PUT") {
			return route.fulfill({
				headers: { ...cors, ETag: '"v2"' },
				json: { ...post, ...req.postDataJSON() },
			});
		}
		return route.fulfill({ headers: { ...cors, ETag: '"v1"' }, json: post });
	});
	return calls;
}

async function openPost(page: Page) {
	await page.goto("/admin");
	await page.getByRole("link", { name: "Opening day" }).click();
	await expect(page.getByLabel("Headline")).toHaveValue("Opening day");
}

const preview = (page: Page) => page.getByRole("region", { name: "Preview" });
const toast = (page: Page) =>
	page.getByRole("status").filter({ hasText: "Post deleted" });

test("Preview shows the post as members will see it, with unsaved changes", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page);
	await openPost(page);
	await page.getByLabel("Headline").fill("Opening day on the green");

	await page.getByRole("button", { name: "Preview" }).click();
	await expect(page.getByLabel("Headline")).toBeHidden();
	await expect(
		preview(page).getByRole("heading", { name: "Opening day on the green" }),
	).toBeVisible();
	await expect(preview(page)).toContainText("The green opens for the season.");
	await expect(
		preview(page).getByRole("heading", { name: "How it went" }),
	).toBeVisible();
	await expect(preview(page).locator("strong")).toHaveText("sunny");
	await expect(preview(page).getByRole("img")).toHaveAttribute(
		"alt",
		"Two bowlers on the green",
	);
	// The site's own sanitising applies, so the preview can't run a post's HTML.
	expect(await page.evaluate(() => "hacked" in window)).toBe(false);

	await page.getByRole("button", { name: "Back to editing" }).click();
	await expect(page.getByLabel("Headline")).toHaveValue(
		"Opening day on the green",
	);
	await expect(preview(page)).toBeHidden();
});

test("a post without a cover previews with the club photo", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page, {
		...stored,
		coverImageKey: null,
		coverAlt: null,
		coverUrl: undefined,
	});
	await openPost(page);
	await page.getByRole("button", { name: "Preview" }).click();
	const cover = preview(page).locator("img");
	await expect(cover).toHaveAttribute("src", /two-crown-green-bowls/);
	await expect(cover).toHaveAttribute("alt", "");
});

test("Delete post removes it at once and offers Undo", async ({ page }) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);

	await page.getByRole("button", { name: "Delete post" }).click();
	await expect(
		page.getByRole("heading", { level: 1, name: "News posts" }),
	).toBeVisible();
	expect(new URL(page.url()).search).toBe("");
	expect(calls).toContain("DELETE /api/admin/posts/01LIVE");
	await expect(page.getByText("No news posts yet.")).toBeVisible();
	await expect(toast(page)).toBeVisible();

	// The browser's back button doesn't return to the deleted post.
	await page.goBack();
	await expect(page.getByLabel("Headline")).toBeHidden();
});

test("Undo brings the post back", async ({ page }) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);
	await page.getByRole("button", { name: "Delete post" }).click();
	await toast(page).getByRole("button", { name: "Undo" }).click();

	await expect
		.poll(() => calls)
		.toContain("POST /api/admin/posts/01LIVE/restore");
	await expect(page.getByRole("link", { name: "Opening day" })).toBeVisible();
	await expect(toast(page)).toBeHidden();
});

test("after six seconds the toast goes and the post stays deleted", async ({
	page,
}) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);
	await page.getByRole("button", { name: "Delete post" }).click();
	await expect(toast(page)).toBeVisible();
	await page.mouse.move(0, 0); // nowhere near the toast, which pauses on hover
	await expect(toast(page)).toBeHidden({ timeout: 8000 });
	expect(calls.filter((c) => c.endsWith("/restore"))).toHaveLength(0);
});

test("changes are saved before deleting, so Undo brings them back too", async ({
	page,
}) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);
	await page.getByLabel("Headline").fill("Opening day, edited");
	await page.getByRole("button", { name: "Delete post" }).click();
	await expect(toast(page)).toBeVisible();
	expect(calls.indexOf("PUT /api/admin/posts/01LIVE")).toBeLessThan(
		calls.indexOf("DELETE /api/admin/posts/01LIVE"),
	);
});

test("a new post that hasn't been saved has nothing to delete", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin?post=new");
	await expect(page.getByLabel("Headline")).toBeVisible();
	await expect(page.getByRole("button", { name: "Delete post" })).toBeHidden();
});

test("the toast waits while the pointer is on it, so there's time to reach Undo", async ({
	page,
}) => {
	test.slow();
	await stubAuth0(page);
	await stubApi(page);
	await openPost(page);
	await page.getByRole("button", { name: "Delete post" }).click();
	await toast(page).hover();
	await page.waitForTimeout(7000);
	await expect(toast(page)).toBeVisible();
	await page.mouse.move(0, 0);
	await expect(toast(page)).toBeHidden({ timeout: 8000 });
});
