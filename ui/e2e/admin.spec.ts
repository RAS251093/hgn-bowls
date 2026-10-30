import { expect, type Page, test } from "@playwright/test";
import {
	accessToken,
	auth0,
	refreshToken,
	stubAuth0,
	stubPosts,
} from "./support/admin";

const post = (id: string, title: string, status: "draft" | "published") => ({
	id,
	title,
	status,
	publishedAt: status === "published" ? "2026-04-18T09:00:00.000000Z" : null,
	updatedAt: "2026-04-18T09:00:00.000000Z",
});
const posts = {
	posts: [
		post("1", "Opening day", "published"),
		post("2", "AGM notice", "published"),
		post("3", "Draft fixtures", "draft"),
	],
};

// The page is ready once the posts have loaded: Write a post appears with them.
const signedIn = (page: Page) =>
	expect(page.getByRole("link", { name: "Write a post" })).toBeVisible();

test("a signed-out visitor is sent to Auth0, with PKCE and a refresh token asked for", async ({
	page,
}) => {
	await stubAuth0(page, { signedIn: false });
	await page.goto("/admin");
	await expect(page).toHaveTitle("Auth0 sign in");

	const url = new URL(page.url());
	expect(url.origin).toBe(auth0);
	expect(url.searchParams.get("client_id")).toBe("test-client");
	expect(url.searchParams.get("audience")).toBe(
		"https://houghgreenparkbowls.uk/api",
	);
	expect(url.searchParams.get("code_challenge_method")).toBe("S256");
	expect(url.searchParams.get("scope")?.split(" ")).toContain("offline_access");
	expect(url.searchParams.get("redirect_uri")).toMatch(/\/admin$/);
});

test("after sign-in the page shows who's signed in and reads the API with the token", async ({
	page,
}) => {
	await stubAuth0(page);
	const auth = await stubPosts(page, [{ status: 200, json: posts }]);

	await page.goto("/admin");
	await signedIn(page);
	await expect(page.getByRole("banner")).toContainText("Rob Smith");
	await expect(
		page.getByRole("list", { name: "Posts" }).getByRole("listitem"),
	).toHaveCount(3);
	expect(auth).toEqual([`Bearer ${accessToken}`]);
	// The code and state are tidied out of the address bar.
	expect(new URL(page.url()).search).toBe("");
});

test("tokens are never kept in local storage, session storage or cookies", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubPosts(page);
	await page.goto("/admin");
	await signedIn(page);

	const stored = await page.evaluate(() =>
		JSON.stringify([
			{ ...localStorage },
			{ ...sessionStorage },
			document.cookie,
		]),
	);
	const cookies = JSON.stringify(await page.context().cookies());
	for (const secret of [accessToken, refreshToken, "id_token", "eyJ"]) {
		expect(stored).not.toContain(secret);
		expect(cookies).not.toContain(secret);
	}
});

test("someone without the club admin role is told so, and can sign out", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubPosts(page, [{ status: 403 }]);
	await page.goto("/admin");
	await expect(page.getByRole("alert")).toContainText(
		"isn't set up as a club admin",
	);
	await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
});

test("a refused token is renewed once and the request tried again", async ({
	page,
}) => {
	const stub = await stubAuth0(page);
	const auth = await stubPosts(page, [
		{ status: 401 },
		{ status: 200, json: posts },
	]);
	await page.goto("/admin");
	await expect(
		page.getByRole("list", { name: "Posts" }).getByRole("listitem"),
	).toHaveCount(3);
	expect(auth).toHaveLength(2);
	// The second token came from the refresh token, not another sign-in.
	expect(stub.calls.filter((c) => c === "POST /oauth/token")).toHaveLength(2);
	expect(stub.calls.filter((c) => c === "GET /authorize")).toHaveLength(1);
});

test("sign out revokes the refresh token and ends the Auth0 session", async ({
	page,
}) => {
	const stub = await stubAuth0(page);
	await stubPosts(page);
	await page.route("https://api.test/api/events*", (route) =>
		route.fulfill({ json: { events: [], stale: false } }),
	);
	await page.goto("/admin");
	await signedIn(page);

	await page.getByRole("button", { name: "Sign out" }).click();
	await expect(page).toHaveURL(/\/$/);
	expect(stub.revoked.join()).toContain(refreshToken);
	expect(stub.calls).toContain("GET /v2/logout");
	expect(stub.calls.indexOf("POST /oauth/revoke")).toBeLessThan(
		stub.calls.indexOf("GET /v2/logout"),
	);

	// Nothing is left in the page to sign back in with.
	const authorizes = stub.calls.filter((c) => c === "GET /authorize").length;
	await page.goto("/admin");
	await signedIn(page);
	expect(stub.calls.filter((c) => c === "GET /authorize")).toHaveLength(
		authorizes + 1,
	);
});

test("the admin page is kept out of search engines", async ({ page }) => {
	await stubAuth0(page, { signedIn: false });
	const res = await page.request.get("/admin");
	expect(await res.text()).toContain('<meta name="robots" content="noindex">');
});

test("a reload goes through Auth0 and comes back signed in", async ({
	page,
}) => {
	const stub = await stubAuth0(page);
	await stubPosts(page);
	await page.goto("/admin");
	await signedIn(page);

	await page.reload();
	await signedIn(page);
	expect(stub.calls.filter((c) => c === "GET /authorize")).toHaveLength(2);
	expect(new URL(page.url()).search).toBe("");
});
