// What the signed-in admin may do, read from their access token's
// `permissions` claim (Auth0 RBAC). It only decides what the page shows:
// the API checks the token itself, so a tampered token gains nothing.

/** The permissions in an access token; [] for anything that isn't a JWT carrying them. */
export function permissionsOf(token: string): string[] {
	const payload = token.split(".")[1];
	if (!payload) return [];
	try {
		const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
		const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
		const claims: unknown = JSON.parse(new TextDecoder().decode(bytes));
		if (typeof claims !== "object" || claims === null) return [];
		const { permissions } = claims as { permissions?: unknown };
		if (!Array.isArray(permissions)) return [];
		return permissions.filter((p): p is string => typeof p === "string");
	} catch {
		return [];
	}
}
