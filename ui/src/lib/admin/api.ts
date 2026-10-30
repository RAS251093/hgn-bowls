// Calls to the admin API. Each request carries the signed-in admin's access
// token. Tokens last 10 minutes, so a 401 usually means this one just
// expired: the client gets a fresh one (from the refresh token) and tries
// once more, and only then sends the admin to sign in again.

/** Where access tokens come from (the Auth0 session in the browser). */
export interface Tokens {
	/** An access token; fresh skips the cached one. */
	token(options?: { fresh?: boolean }): Promise<string>;
	/** Sends the admin to Auth0 to sign in again. */
	signIn(): Promise<void>;
}

/** The admin has been sent to sign in; stop what you were doing. */
export class SignInRequired extends Error {
	constructor() {
		super("signing in again");
		this.name = "SignInRequired";
	}
}

export type Api = (path: string, init?: RequestInit) => Promise<Response>;

export function apiClient(
	baseUrl: string,
	tokens: Tokens,
	get: typeof fetch = fetch,
): Api {
	const send = async (path: string, init: RequestInit, fresh: boolean) => {
		let token: string;
		try {
			token = await tokens.token({ fresh });
		} catch {
			// The refresh token has expired or been revoked.
			await tokens.signIn();
			throw new SignInRequired();
		}
		const headers = new Headers(init.headers);
		headers.set("Authorization", `Bearer ${token}`);
		return get(`${baseUrl}${path}`, { ...init, headers });
	};

	return async (path, init = {}) => {
		const res = await send(path, init, false);
		if (res.status !== 401) return res;
		const retry = await send(path, init, true);
		if (retry.status !== 401) return retry;
		await tokens.signIn();
		throw new SignInRequired();
	};
}
