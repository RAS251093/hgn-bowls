import { expect, type Page, test } from "@playwright/test";
import { stubAuth0 } from "./support/admin";

// Publishing saves the post as live and starts a website rebuild; the
// editor then follows the rebuild (GET /api/admin/deploys/latest) until the
// new build is live or has failed. The build polls fast for tests
// (PUBLIC_DEPLOY_POLL_MS).

const api = "https://api.test/api/admin";
const cors = {
	"Access-Control-Allow-Origin": "*",
	"Access-Control-Expose-Headers": "ETag",
};

const draft = {
	id: "01DRAFT",
	slug: null,
	title: "Winter league",
	description: "Fixtures for the winter.",
	bodyMd: "Members only.",
	coverImageKey: null,
	coverAlt: null,
	status: "draft",
	publishedAt: null,
	updatedAt: "2026-10-30T09:00:00.000000Z",
};

type Build = { id: string; status: "building" | "live" | "failed" } | null;

interface Options {
	post?: typeof draft | Record<string, unknown>;
	/** What the hook call does when publishing: started or failed. */
	hook?: "started" | "failed";
	/** Builds the deploys endpoint reports, one per poll; the last repeats. */
	builds?: Build[];
	/** Builds reported after Try again. */
	afterRetry?: Build[];
}

async function stubApi(
	page: Page,
	{
		post = draft,
		hook = "started",
		builds = [
			{ id: "old", status: "live" },
			{ id: "old", status: "live" },
			{ id: "new", status: "building" },
			{ id: "new", status: "live" },
		],
		afterRetry = [
			{ id: "retry", status: "building" },
			{ id: "retry", status: "live" },
		],
	}: Options = {},
) {
	const calls: { method: string; path: string; ifMatch?: string }[] = [];
	let current = { ...post };
	let version = 1;
	let queue = [...builds];
	const next = () => (queue.length > 1 ? queue.shift() : queue[0]) ?? null;

	await page.route(`${api}/**`, async (route) => {
		const req = route.request();
		const path = new URL(req.url()).pathname.replace("/api/admin", "");
		calls.push({
			method: req.method(),
			path,
			ifMatch: (await req.allHeaders())["if-match"],
		});
		const etag = () => ({ ...cors, ETag: `"v${version}"` });

		if (path === "/posts") {
			return route.fulfill({ json: { posts: [current] } });
		}
		if (path === "/deploys/latest") {
			return route.fulfill({ json: { deployment: next() } });
		}
		if (path === "/rebuild") {
			queue = [...afterRetry];
			return route.fulfill({ json: { rebuild: { status: "started" } } });
		}
		if (path.endsWith("/publish")) {
			version++;
			current = {
				...current,
				status: "published",
				slug: "winter-league",
				publishedAt: "2026-10-30T10:00:00.000000Z",
			};
			return route.fulfill({
				headers: etag(),
				json: { post: current, rebuild: { status: hook } },
			});
		}
		if (path.endsWith("/unpublish")) {
			version++;
			current = { ...current, status: "draft" };
			return route.fulfill({
				headers: etag(),
				json: { post: current, rebuild: { status: "started" } },
			});
		}
		if (req.method() === "PUT") {
			version++;
			current = { ...current, ...req.postDataJSON() };
			return route.fulfill({ headers: etag(), json: current });
		}
		return route.fulfill({ headers: etag(), json: current });
	});
	return calls;
}

const panel = (page: Page) => page.locator("[data-publish-status]");

async function openPost(page: Page) {
	await page.goto("/admin?post=01DRAFT");
	await expect(page.getByLabel("Headline")).toBeVisible();
}

test("publishing a draft follows the website update until it's live", async ({
	page,
}) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);
	await expect(page.locator("[data-editor-badge]")).toHaveText("Draft");

	await page.getByRole("button", { name: "Publish post" }).click();
	await expect(panel(page)).toContainText("Updating the website");
	await expect(panel(page)).toContainText("Live on the website");
	await expect(
		panel(page).getByRole("link", { name: "View the post" }),
	).toHaveAttribute("href", "/news/winter-league");
	expect(calls.map((c) => `${c.method} ${c.path}`)).toContain(
		"POST /posts/01DRAFT/publish",
	);
	await expect(page.locator("[data-editor-badge]")).toHaveText("Published");
	// A live post's button sends later edits to the website.
	await expect(
		page.getByRole("button", { name: "Update the website" }),
	).toBeVisible();
});

test("it isn't Live until the new build is, not the one from before", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page, {
		builds: [
			{ id: "old", status: "live" }, // before publishing
			{ id: "old", status: "live" },
			{ id: "old", status: "live" },
			{ id: "old", status: "live" },
			{ id: "new", status: "building" },
		],
	});
	await openPost(page);
	await page.getByRole("button", { name: "Publish post" }).click();
	await page.waitForTimeout(1500);
	await expect(panel(page)).toContainText("Updating the website");
	await expect(panel(page)).not.toContainText("Live");
});

test("a failed update says so, and Try again starts another", async ({
	page,
}) => {
	await stubAuth0(page);
	const calls = await stubApi(page, {
		builds: [
			{ id: "old", status: "live" },
			{ id: "new", status: "failed" },
		],
	});
	await openPost(page);
	await page.getByRole("button", { name: "Publish post" }).click();
	await expect(panel(page)).toContainText("The website update failed");
	await expect(panel(page)).toContainText("Your post is saved");

	await panel(page).getByRole("button", { name: "Try again" }).click();
	await expect(panel(page)).toContainText("Live on the website");
	expect(calls.map((c) => `${c.method} ${c.path}`)).toContain("POST /rebuild");
});

test("if the update doesn't start, the post is still saved and Try again is offered", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page, { hook: "failed" });
	await openPost(page);
	await page.getByRole("button", { name: "Publish post" }).click();
	await expect(panel(page)).toContainText(
		"Saved, but the website didn't start updating",
	);
	await panel(page).getByRole("button", { name: "Try again" }).click();
	await expect(panel(page)).toContainText("Live on the website");
});

test("edits after publishing save with the new version, not a conflict", async ({
	page,
}) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);
	await page.getByRole("button", { name: "Publish post" }).click();
	await expect(panel(page)).toContainText("Live on the website");

	await page.getByLabel("Headline").fill("Winter league fixtures");
	await expect(
		page.getByRole("status").filter({ hasText: "Saved" }),
	).toBeVisible();
	const put = calls.findLast((c) => c.method === "PUT");
	expect(put?.ifMatch).toBe('"v2"');
	await expect(page.getByRole("alert")).toHaveCount(0);
});

test("unsaved edits are saved before publishing", async ({ page }) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);
	await page.getByLabel("Headline").fill("Winter league 2026");
	await page.getByRole("button", { name: "Publish post" }).click();
	await expect(panel(page)).toContainText("Updating the website");
	// The panel says so just before the publish request goes, so wait for it.
	const order = () => calls.map((c) => `${c.method} ${c.path}`);
	await expect.poll(order).toContain("POST /posts/01DRAFT/publish");
	expect(order().indexOf("PUT /posts/01DRAFT")).toBeGreaterThanOrEqual(0);
	expect(order().indexOf("PUT /posts/01DRAFT")).toBeLessThan(
		order().indexOf("POST /posts/01DRAFT/publish"),
	);
});

test("a post with problems can't be published until they're fixed", async ({
	page,
}) => {
	await stubAuth0(page);
	const calls = await stubApi(page);
	await openPost(page);
	await page.getByLabel("Headline").fill("");
	await page.getByRole("button", { name: "Publish post" }).click();
	await expect(panel(page)).toContainText("Fix the highlighted fields");
	expect(calls.some((c) => c.path.endsWith("/publish"))).toBe(false);
});

test("Unpublish takes a live post off the website", async ({ page }) => {
	await stubAuth0(page);
	const calls = await stubApi(page, {
		post: {
			...draft,
			status: "published",
			slug: "winter-league",
			publishedAt: "2026-10-30T10:00:00.000000Z",
		},
	});
	await openPost(page);
	await expect(page.locator("[data-editor-badge]")).toHaveText("Published");

	await page.getByRole("button", { name: "Unpublish" }).click();
	await expect(panel(page)).toContainText("Taking the post off the website");
	await expect(panel(page)).toContainText("The post is off the website");
	expect(calls.map((c) => `${c.method} ${c.path}`)).toContain(
		"POST /posts/01DRAFT/unpublish",
	);
	await expect(page.locator("[data-editor-badge]")).toHaveText("Draft");
	await expect(
		page.getByRole("button", { name: "Publish post" }),
	).toBeVisible();
});

test("a new post can be published once it's been saved", async ({ page }) => {
	await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin?post=new");
	await expect(page.getByLabel("Headline")).toBeVisible();
	await expect(page.getByRole("button", { name: "Publish post" })).toBeHidden();
});
