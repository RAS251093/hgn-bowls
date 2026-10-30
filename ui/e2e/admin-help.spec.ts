import { expect, type Page, test } from "@playwright/test";
import { stubAuth0 } from "./support/admin";

// The guides are a view of the admin page (/admin?help), so opening them
// never reloads the page or loses work in the editor.

const api = "https://api.test/api/admin/posts";
const cors = {
	"Access-Control-Allow-Origin": "*",
	"Access-Control-Expose-Headers": "ETag",
};
const post = {
	id: "01LIVE",
	slug: "opening-day",
	title: "Opening day",
	description: "",
	bodyMd: "We opened.",
	coverImageKey: null,
	coverAlt: null,
	status: "draft",
	publishedAt: null,
	updatedAt: "2026-10-30T09:00:00.000000Z",
};

async function stubApi(page: Page) {
	const saves: string[] = [];
	await page.route(api, (route) => route.fulfill({ json: { posts: [post] } }));
	await page.route(`${api}/*`, (route) => {
		if (route.request().method() === "PUT") {
			saves.push(route.request().postDataJSON().title);
		}
		return route.fulfill({ headers: { ...cors, ETag: '"v1"' }, json: post });
	});
	return saves;
}

const help = (page: Page) =>
	page.getByRole("heading", { level: 1, name: "Help for club admins" });

test("Help in the header opens both guides, without going back to Auth0", async ({
	page,
}) => {
	const auth = await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin");
	await page.getByRole("banner").getByRole("link", { name: "Help" }).click();

	await expect(help(page)).toBeVisible();
	expect(new URL(page.url()).search).toBe("?help");
	await expect(
		page.getByRole("heading", { level: 2, name: "Writing a news post" }),
	).toBeVisible();
	await expect(
		page.getByRole("heading", { level: 2, name: "Adding an event" }),
	).toBeVisible();
	expect(auth.calls.filter((c) => c === "GET /authorize")).toHaveLength(1);

	// Back to the posts from the guides.
	await page.getByRole("link", { name: "All posts" }).click();
	await expect(
		page.getByRole("heading", { level: 1, name: "News posts" }),
	).toBeVisible();
});

test("the events note on the list leads to the events guide", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin");
	await page.getByRole("link", { name: "How to add an event" }).click();
	const guide = page.getByRole("heading", {
		level: 2,
		name: "Adding an event",
	});
	await expect(guide).toBeFocused();
	await expect(guide).toBeInViewport();
	expect(new URL(page.url()).search).toBe("?help=events");
});

test("the editor links to the posts guide, saving first, and Back returns", async ({
	page,
}) => {
	await stubAuth0(page);
	const saves = await stubApi(page);
	await page.goto("/admin?post=01LIVE");
	await page.getByLabel("Headline").fill("Opening day, edited");
	await page.getByRole("link", { name: "How to write a post" }).click();

	await expect(
		page.getByRole("heading", { level: 2, name: "Writing a news post" }),
	).toBeFocused();
	expect(saves).toContain("Opening day, edited");

	await page.goBack();
	await expect(page.getByLabel("Headline")).toBeVisible();
});

test("a link straight to a guide opens it after signing in", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin?help=events");
	await expect(help(page)).toBeVisible();
	await expect(
		page.getByRole("heading", { level: 2, name: "Adding an event" }),
	).toBeInViewport();
});

test("the guides cover what admins need", async ({ page }) => {
	await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin?help");
	const posts = page.getByRole("region", { name: "Writing a news post" });
	for (const step of [
		"Headline",
		"Describe the photo",
		"YouTube",
		"Saved",
		"Preview",
		"Publish post",
		"Update the website",
		"Undo",
	]) {
		await expect(posts).toContainText(step);
	}
	const events = page.getByRole("region", { name: "Adding an event" });
	for (const step of ["Google Calendar", "Location", "Description", "repeat"]) {
		await expect(events).toContainText(step);
	}
	await expect(
		events.getByRole("link", { name: "Open Google Calendar" }),
	).toHaveAttribute("href", /^https:\/\/calendar\.google\.com\//);
});
