// Following a website rebuild after publishing (GET /api/admin/deploys/latest).

export interface Deployment {
	id: string;
	status: "building" | "live" | "failed";
}

export type DeployState = "waiting" | "building" | "live" | "failed";

/**
 * Where the rebuild has got to. Right after publishing, Cloudflare's newest
 * build is still the one from before, which is live, so the state stays
 * "waiting" until a build with a different id appears.
 */
export function deployState(
	before: string | null,
	latest: Deployment | null,
): DeployState {
	if (!latest || latest.id === before) return "waiting";
	return latest.status;
}
