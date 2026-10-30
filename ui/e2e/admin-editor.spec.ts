import { expect, type Page, type Request, test } from "@playwright/test";
import { stubAuth0 } from "./support/admin";

// The editor is a view of the one admin page (/admin?post=<id>), so
// opening it from the list never reloads the page or goes back to Auth0.

const api = "https://api.test/api/admin/posts";

const stored = {
	id: "01LIVE",
	slug: "opening-day",
	title: "Opening day",
	description: "The green opens for the season.",
	bodyMd: "We opened the green.\n\n## How it went\n\nIt was **sunny**.",
	coverImageKey: "posts/01COVER.jpg",
	coverAlt: "Two bowlers on the green",
	status: "published",
	publishedAt: "2025-04-05T09:00:00.000000Z",
	createdAt: "2025-04-05T09:00:00.000000Z",
	updatedAt: "2025-04-05T09:00:00.000000Z",
};

interface Saved {
	method: string;
	url: string;
	ifMatch: string | undefined;
	body: Record<string, string | null>;
}

// As the real API sends: the browser can only read ETag if it's exposed.
const cors = {
	"Access-Control-Allow-Origin": "*",
	"Access-Control-Expose-Headers": "ETag",
};

/** Stubs the posts API: the list, one post, and saves (answered with saveStatus). */
async function stubApi(
	page: Page,
	{ saveStatus = 200 }: { saveStatus?: number } = {},
) {
	const saves: Saved[] = [];
	let version = 1;
	// headers() leaves some out; allHeaders() has If-Match.
	const record = async (req: Request) =>
		saves.push({
			method: req.method(),
			url: req.url(),
			ifMatch: (await req.allHeaders())["if-match"],
			body: req.postDataJSON(),
		});
	await page.route(api, async (route) => {
		const req = route.request();
		if (req.method() === "POST") {
			await record(req);
			return route.fulfill({
				status: 201,
				headers: { ...cors, ETag: `"new-v${version}"` },
				json: {
					...stored,
					...req.postDataJSON(),
					id: "01NEW",
					status: "draft",
				},
			});
		}
		return route.fulfill({
			json: { posts: [{ ...stored }] },
		});
	});
	await page.route(`${api}/*`, async (route) => {
		const req = route.request();
		const id = new URL(req.url()).pathname.split("/").pop();
		if (req.method() === "PUT") {
			await record(req);
			version++;
			return route.fulfill({
				status: saveStatus,
				headers: { ...cors, ETag: `"${id}-v${version}"` },
				json:
					saveStatus === 200 ? { ...stored, ...req.postDataJSON(), id } : {},
			});
		}
		return route.fulfill({
			headers: { ...cors, ETag: `"${id}-v1"` },
			json: { ...stored, id },
		});
	});
	return saves;
}

const editor = (page: Page) => page.getByRole("textbox", { name: "Post" });
const status = (page: Page) => page.getByRole("status");

// Puts the cursor at the very end of the post (the keys differ on a Mac).
// The click is held for 100ms, as a person's is: ProseMirror misses where
// an instant synthetic click into an unfocused editor put the cursor.
async function toEndOfPost(page: Page) {
	await editor(page).getByText("It was").click({ delay: 100 });
	await page.keyboard.press(
		process.platform === "darwin" ? "Meta+ArrowDown" : "Control+End",
	);
}

async function openPost(page: Page) {
	await page.goto("/admin");
	await page.getByRole("link", { name: "Opening day" }).click();
	await expect(
		page.getByRole("heading", { level: 1, name: "Edit post" }),
	).toBeVisible();
}

test("opening a post shows it as rich text, without going back to Auth0", async ({
	page,
}) => {
	const auth = await stubAuth0(page);
	await stubApi(page);
	await openPost(page);

	expect(new URL(page.url()).search).toBe("?post=01LIVE");
	expect(auth.calls.filter((c) => c === "GET /authorize")).toHaveLength(1);
	await expect(page.getByLabel("Headline")).toHaveValue("Opening day");
	await expect(page.getByLabel("Description")).toHaveValue(
		"The green opens for the season.",
	);
	// Formatting shows as formatting; the admin never sees Markdown.
	await expect(
		editor(page).getByRole("heading", { level: 2, name: "How it went" }),
	).toBeVisible();
	await expect(editor(page).locator("strong")).toHaveText("sunny");
	await expect(editor(page)).not.toContainText("##");
	await expect(editor(page)).not.toContainText("**");
});

test("changes save themselves, as Markdown, with the version they were loaded at", async ({
	page,
}) => {
	await stubAuth0(page);
	const saves = await stubApi(page);
	await openPost(page);

	await toEndOfPost(page);
	await page.keyboard.type(" Everyone stayed for tea.");
	await expect(status(page)).toContainText("Saved");

	expect(saves).toHaveLength(1);
	expect(saves[0].method).toBe("PUT");
	expect(saves[0].url).toBe(`${api}/01LIVE`);
	expect(saves[0].ifMatch).toBe('"01LIVE-v1"');
	expect(saves[0].body.bodyMd).toBe(
		"We opened the green.\n\n## How it went\n\nIt was **sunny**. Everyone stayed for tea.",
	);
	// The cover isn't edited here yet, so it goes back as it came.
	expect(saves[0].body.coverImageKey).toBe("posts/01COVER.jpg");
	expect(saves[0].body.coverAlt).toBe("Two bowlers on the green");

	// The next save sends the version the last one returned.
	await page.getByLabel("Headline").fill("Opening day on the green");
	await expect(status(page)).toContainText("Saved");
	await expect.poll(() => saves.length).toBe(2);
	expect(saves[1].ifMatch).toBe('"01LIVE-v2"');
	expect(saves[1].body.title).toBe("Opening day on the green");
});

test("the toolbar formats text without the admin typing any Markdown", async ({
	page,
}) => {
	await stubAuth0(page);
	const saves = await stubApi(page);
	await openPost(page);

	await toEndOfPost(page);
	await page.keyboard.press("Enter");
	await page.getByRole("button", { name: "Subheading", exact: true }).click();
	await page.keyboard.type("Next match");
	await page.keyboard.press("Enter");
	await page.getByRole("button", { name: "Bulleted list" }).click();
	await page.keyboard.type("Bring flat shoes");
	await page.keyboard.press("Enter");
	await page.keyboard.type("Bring a jumper");
	await page.keyboard.press("Enter");
	await page.keyboard.press("Enter"); // leaves the list
	await page.getByRole("button", { name: "Bold" }).click();
	await page.keyboard.type("See you there");

	// Saves can happen mid-typing; the last one has everything.
	await expect
		.poll(() => saves.at(-1)?.body.bodyMd)
		.toMatch(
			/## Next match\n\n- Bring flat shoes\n- Bring a jumper\n\n\*\*See you there\*\*$/,
		);
});

test("a link is added to the selected words", async ({ page }) => {
	await stubAuth0(page);
	const saves = await stubApi(page);
	await openPost(page);

	// Three clicks select the paragraph.
	await editor(page).getByText("We opened the green.").click({ clickCount: 3 });
	await page.getByRole("button", { name: "Link" }).click();
	await page.getByLabel("Link address").fill("example.org/opening");
	await page.getByRole("button", { name: "Add link" }).click();

	await expect(status(page)).toContainText("Saved");
	expect(saves.at(-1)?.body.bodyMd).toMatch(
		/^\[We opened the green\.\]\(https:\/\/example\.org\/opening\)/,
	);
});

test("Write a post creates the post on its first save, then keeps editing it", async ({
	page,
}) => {
	await stubAuth0(page);
	const saves = await stubApi(page);
	await page.goto("/admin");
	await page.getByRole("link", { name: "Write a post" }).click();
	await expect(
		page.getByRole("heading", { level: 1, name: "Write a post" }),
	).toBeVisible();

	await editor(page).click({ delay: 100 });
	await page.keyboard.type("Members only.");
	// No headline yet, so nothing is created.
	await expect(status(page)).toContainText("Add a headline to save");
	expect(saves).toHaveLength(0);

	await page.getByLabel("Headline").fill("Winter league");
	await expect(status(page)).toContainText("Saved");
	expect(saves[0].method).toBe("POST");
	expect(saves[0].body).toMatchObject({
		title: "Winter league",
		bodyMd: "Members only.",
	});
	await expect(page).toHaveURL(/\?post=01NEW$/);

	await page.getByLabel("Description").fill("Fixtures for the winter.");
	await expect.poll(() => saves.length).toBe(2);
	expect(saves[1]).toMatchObject({ method: "PUT", ifMatch: '"new-v1"' });
	expect(saves[1].url).toBe(`${api}/01NEW`);
});

test("limits are shown beside the field, and nothing is saved past them", async ({
	page,
}) => {
	await stubAuth0(page);
	const saves = await stubApi(page);
	await openPost(page);

	const headline = page.getByLabel("Headline");
	await headline.fill("x".repeat(121));
	await expect(headline).toHaveAttribute("aria-invalid", "true");
	await expect(
		page.getByText("Keep the headline to 120 characters or fewer"),
	).toBeVisible();
	await expect(status(page)).toContainText("Not saved");

	await headline.fill("");
	await expect(page.getByText("Add a headline", { exact: true })).toBeVisible();

	await headline.fill("Opening day");
	await expect(headline).not.toHaveAttribute("aria-invalid", "true");

	await page.getByLabel("Description").fill("y".repeat(301));
	await expect(
		page.getByText("Keep the description to 300 characters or fewer"),
	).toBeVisible();
	await page.waitForTimeout(2000);
	expect(saves.map((s) => s.body.description?.length ?? 0)).not.toContain(301);
});

test("if someone else saved the post first, the editor stops and says what to do", async ({
	page,
}) => {
	await stubAuth0(page);
	const saves = await stubApi(page, { saveStatus: 409 });
	await openPost(page);

	await page.getByLabel("Headline").fill("My version");
	await expect(page.getByRole("alert")).toContainText(
		"Someone else has changed this post since you opened it",
	);
	await page.getByLabel("Headline").fill("My version, again");
	await page.waitForTimeout(2000);
	expect(saves).toHaveLength(1);
});

test("All posts goes back to the list, saving first", async ({ page }) => {
	await stubAuth0(page);
	const saves = await stubApi(page);
	await openPost(page);

	await page.getByLabel("Headline").fill("Opening day, updated");
	await page.getByRole("link", { name: "All posts" }).click();
	await expect(
		page.getByRole("heading", { level: 1, name: "News posts" }),
	).toBeVisible();
	expect(new URL(page.url()).search).toBe("");
	expect(saves.at(-1)?.body.title).toBe("Opening day, updated");

	// The browser's back button returns to the editor.
	await page.goBack();
	await expect(
		page.getByRole("heading", { level: 1, name: "Edit post" }),
	).toBeVisible();
});

test("a link straight to a post opens it after signing in", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin?post=01LIVE");
	await expect(
		page.getByRole("heading", { level: 1, name: "Edit post" }),
	).toBeVisible();
	await expect(page.getByLabel("Headline")).toHaveValue("Opening day");
	expect(new URL(page.url()).search).toBe("?post=01LIVE");
});

test("while a post loads, placeholders show where its fields will be", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page);
	let release = () => {};
	const answered = new Promise<void>((r) => {
		release = r;
	});
	await page.route(`${api}/01LIVE`, async (route) => {
		await answered;
		await route.fallback();
	});
	await page.goto("/admin");
	await page.getByRole("link", { name: "Opening day" }).click();

	const loading = page.locator("[data-editor-loading]");
	await expect(loading).toBeVisible();
	await expect(loading.locator("[aria-hidden=true]")).toBeVisible();
	await expect(
		page.getByRole("status").filter({ hasText: "Loading the post" }),
	).toHaveCount(1);
	await expect(page.getByLabel("Headline")).toBeHidden();

	release();
	await expect(page.getByLabel("Headline")).toHaveValue("Opening day");
	await expect(loading).toBeHidden();
});

test("a new post needs no loading, so it shows no placeholders", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubApi(page);
	await page.goto("/admin?post=new");
	await expect(page.getByLabel("Headline")).toBeVisible();
	await expect(page.locator("[data-editor-loading]")).toBeHidden();
});
