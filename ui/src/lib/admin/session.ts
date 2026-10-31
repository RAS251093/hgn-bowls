// The admin's Auth0 session in the browser. Sign-in uses PKCE with a
// refresh token, and every token stays in memory (in the SDK's worker), so
// nothing can be read from storage or cookies.
//
// Memory doesn't survive a reload, so a reload sends the admin to Auth0,
// which sees its own session cookie and sends them straight back. That's a
// top-level redirect, so it works in Safari too, which blocks the hidden
// iframe the SDK would otherwise use.
import { type Auth0Client, createAuth0Client } from "@auth0/auth0-spa-js";
import { auth0 } from "../../config";
import type { Tokens } from "./api";
import { permissionsOf } from "./permissions";

export interface Session extends Tokens {
	name: string;
	/** Their Auth0 user id and email address, to find them in a list of admins. */
	sub?: string;
	email?: string;
	/**
	 * Whether their access token grants a permission, such as
	 * "manage:admins". For what the page shows only: the API decides.
	 */
	can(permission: string): boolean;
	/** Revokes the refresh token, then ends the Auth0 session and comes back to the home page. */
	signOut(): Promise<void>;
}

/**
 * Finishes a sign-in that has just come back from Auth0, or signs the admin
 * in. Resolves to null while the browser is on its way to Auth0.
 */
export async function startSession(): Promise<Session | null> {
	const here = `${location.origin}/admin`;
	const client = await createAuth0Client({
		domain: auth0.domain,
		clientId: auth0.clientId,
		cacheLocation: "memory",
		useRefreshTokens: true,
		authorizationParams: {
			audience: auth0.audience,
			scope: "openid profile email offline_access",
			redirect_uri: here,
		},
	});

	const params = new URLSearchParams(location.search);
	if (params.has("state") && (params.has("code") || params.has("error"))) {
		let back = "/admin";
		try {
			const { appState } = await client.handleRedirectCallback<AppState>();
			back = adminPath(appState?.returnTo);
		} finally {
			history.replaceState(null, "", back);
		}
	}

	const signIn = () =>
		client.loginWithRedirect<AppState>({
			appState: { returnTo: location.pathname + location.search },
		});
	if (!(await client.isAuthenticated())) {
		await signIn();
		return null;
	}
	return session(client, signIn);
}

// Where to land after signing in: the admin view the admin asked for.
interface AppState {
	returnTo?: string;
}

/** Only ever an admin address on this site, never somewhere else. */
function adminPath(path: string | undefined): string {
	return path && /^\/admin\/?(\?[\w=&%.-]*)?$/.test(path) ? path : "/admin";
}

async function session(
	client: Auth0Client,
	signIn: () => Promise<void>,
): Promise<Session> {
	const user = await client.getUser();
	// Read once at sign-in; a permission given later shows after the next one.
	let permissions: string[] = [];
	try {
		permissions = permissionsOf((await client.getTokenSilently()) ?? "");
	} catch {
		// Nothing extra to show; the API still answers for itself.
	}
	return {
		name: user?.name || user?.email || "you",
		sub: user?.sub,
		email: user?.email,
		can: (permission) => permissions.includes(permission),
		async token({ fresh = false } = {}) {
			const token = await client.getTokenSilently({
				cacheMode: fresh ? "off" : "on",
			});
			if (!token) throw new Error("no access token");
			return token;
		},
		signIn,
		async signOut() {
			try {
				await client.revokeRefreshToken();
			} catch {
				// Signing out of Auth0 still ends the session; the refresh token
				// then expires within a day unused.
			}
			await client.logout({
				logoutParams: { returnTo: `${location.origin}/` },
			});
		},
	};
}
