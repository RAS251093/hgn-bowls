import type { Page } from "@playwright/test";

// The build points the admin page at a stub Auth0 tenant (https://auth.test)
// and API (https://api.test). The stub signs anyone in at once, as Auth0
// does when its own session cookie is still there.

export const auth0 = "https://auth.test";
export const refreshToken = "stub-refresh-token";

const b64url = (o: object) =>
	Buffer.from(JSON.stringify(o)).toString("base64url");

/**
 * An access token shaped like Auth0's, carrying the admin's permissions
 * (the page reads them to decide what to show). Unsigned: only the stub
 * API sees it.
 */
export function accessTokenFor(permissions: string[]): string {
	return [
		b64url({ alg: "RS256", typ: "JWT" }),
		b64url({
			iss: `${auth0}/`,
			sub: "auth0|rob",
			aud: "https://houghgreenparkbowls.uk/api",
			permissions,
		}),
		"stub-signature",
	].join(".");
}

/** The access token an admin who can write posts, and nothing more, gets. */
export const accessToken = accessTokenFor(["manage:posts"]);

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

export interface Stub {
	/** Every request to Auth0, in order, as "METHOD /path". */
	calls: string[];
	/** Bodies POSTed to /oauth/revoke. */
	revoked: string[];
}

/**
 * Stubs Auth0. With signedIn false, /authorize shows a sign-in page instead
 * of returning at once. The access token grants permissions (by default
 * just manage:posts).
 */
export async function stubAuth0(
	page: Page,
	{ signedIn = true, permissions = ["manage:posts"] } = {},
): Promise<Stub> {
	const token = accessTokenFor(permissions);
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
						access_token: token,
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
export async function stubPosts(
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
