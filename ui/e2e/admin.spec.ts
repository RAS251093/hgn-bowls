import { expect, type Page, test } from "@playwright/test";

// The build points the admin page at a stub Auth0 tenant (https://auth.test)
// and API (https://api.test). The stub signs anyone in at once, as Auth0
// does when its own session cookie is still there.

const auth0 = "https://auth.test";
const accessToken = "stub-access-token";
const refreshToken = "stub-refresh-token";

const b64url = (o: object) =>
	Buffer.from(JSON.stringify(o)).toString("base64url");

// An ID token the SDK accepts: it checks the claims, not the signature.
function idToken(nonce: string) {
	const now = Math.floor(Date.now() / 1000);
	return [
		b64url({ alg: "RS256", typ: "JWT" }),
		b64url({
			iss: `${auth0}/`,
			aud: "test-client",
			sub: "auth0|rob",
			name: "Rob Smith",
			email: "rob@example.org",
			nonce,
			iat: now,
			exp: now + 3600,
		}),
		"signature",
	].join(".");
}

interface Stub {
	/** Every request to Auth0, in order, as "METHOD /path". */
	calls: string[];
	/** Bodies POSTed to /oauth/revoke. */
	revoked: string[];
}

/** Stubs Auth0. With signedIn false, /authorize shows a sign-in page instead of returning at once. */
async function stubAuth0(page: Page, { signedIn = true } = {}): Promise<Stub> {
	const stub: Stub = { calls: [], revoked: [] };
	let nonce = "";
	// Context routes also see the SDK's worker, which makes the token calls.
	await page.context().route(`${auth0}/**`, async (route) => {
		const req = route.request();
		const url = new URL(req.url());
		stub.calls.push(`${req.method()} ${url.pathname}`);
		const cors = {
			"Access-Control-Allow-Origin": "*",
			"Access-Control-Allow-Headers": "*",
		};
		if (req.method() === "OPTIONS") {
			return route.fulfill({ status: 204, headers: cors });
		}
		switch (url.pathname) {
			case "/authorize": {
				if (!signedIn) {
					return route.fulfill({
						contentType: "text/html",
						body: "<title>Auth0 sign in</title><h1>Sign in</h1>",
					});
				}
				nonce = url.searchParams.get("nonce") ?? "";
				const back = new URL(url.searchParams.get("redirect_uri") ?? "");
				back.searchParams.set("code", "stub-code");
				back.searchParams.set("state", url.searchParams.get("state") ?? "");
				return route.fulfill({
					status: 302,
					headers: { Location: back.toString() },
				});
			}
			case "/oauth/token":
				return route.fulfill({
					headers: cors,
					json: {
						access_token: accessToken,
						refresh_token: refreshToken,
						id_token: idToken(nonce),
						token_type: "Bearer",
						expires_in: 600,
						scope: "openid profile email offline_access",
					},
				});
			case "/oauth/revoke":
				stub.revoked.push(req.postData() ?? "");
				return route.fulfill({ status: 200, headers: cors, body: "" });
			case "/v2/logout":
				return route.fulfill({
					status: 302,
					headers: { Location: url.searchParams.get("returnTo") ?? "/" },
				});
		}
		return route.fulfill({ status: 404, headers: cors });
	});
	return stub;
}

/** Stubs the admin posts list; returns the Authorization headers it saw. */
async function stubPosts(
	page: Page,
	answers: { status: number; json?: object }[] = [
		{ status: 200, json: { posts: [] } },
	],
) {
	const seen: string[] = [];
	await page.route("https://api.test/api/admin/posts", (route) => {
		seen.push(route.request().headers().authorization ?? "");
		const answer = answers[Math.min(seen.length - 1, answers.length - 1)];
		return route.fulfill({ status: answer.status, json: answer.json ?? {} });
	});
	return seen;
}

const posts = {
	posts: [
		{ id: "1", title: "Opening day", status: "published" },
		{ id: "2", title: "AGM notice", status: "published" },
		{ id: "3", title: "Draft fixtures", status: "draft" },
	],
};

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
	await expect(
		page.getByRole("heading", { level: 1, name: "Club admin" }),
	).toBeVisible();
	await expect(page.getByText("Signed in as Rob Smith")).toBeVisible();
	await expect(page.getByText("2 published posts and 1 draft")).toBeVisible();
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
	await expect(page.getByText("Signed in as Rob Smith")).toBeVisible();

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
	await expect(page.getByText("2 published posts and 1 draft")).toBeVisible();
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
	await expect(page.getByText("Signed in as Rob Smith")).toBeVisible();

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
	await expect(page.getByText("Signed in as Rob Smith")).toBeVisible();
	expect(stub.calls.filter((c) => c === "GET /authorize")).toHaveLength(
		authorizes + 1,
	);
});

test("the admin page is kept out of search engines", async ({ page }) => {
	await stubAuth0(page, { signedIn: false });
	const res = await page.request.get("/admin");
	expect(await res.text()).toContain('<meta name="robots" content="noindex">');
});
