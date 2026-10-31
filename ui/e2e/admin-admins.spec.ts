import { expect, type Page, test } from "@playwright/test";
import { stubAuth0, stubPosts } from "./support/admin";

// The admins view (/admin?admins): only people whose token grants
// manage:admins see the way to it. It lists who can sign in, invites
// someone new, sends an invitation again, and removes access with Undo.

const api = "https://api.test/api/admin/admins";
const cors = {
	"Access-Control-Allow-Origin": "*",
	"Access-Control-Allow-Headers": "*",
	"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE",
};
const managers = ["manage:posts", "manage:admins"];

interface Admin {
	id: string;
	name: string;
	email: string;
	status: "invited" | "active";
	lastLogin: string | null;
}

const rob: Admin = {
	id: "auth0|rob",
	name: "Rob Smith",
	email: "rob@example.org",
	status: "active",
	lastLogin: "2026-04-18T09:00:00Z",
};
const ann: Admin = {
	id: "auth0|ann",
	name: "Ann Jones",
	email: "ann@example.org",
	status: "invited",
	lastLogin: null,
};
const cat: Admin = {
	id: "auth0|cat",
	name: "Cat Moss",
	email: "cat@example.org",
	status: "active",
	lastLogin: "2025-02-01T12:00:00Z",
};

interface Answer {
	status: number;
	json?: object;
}

/** Stubs the admins API; answers can be swapped per call. Returns what it saw. */
async function stubAdmins(
	page: Page,
	{
		list = { status: 200, json: { admins: [ann, cat, rob] } } as Answer,
		create = undefined as Answer | undefined,
		invite = { status: 200, json: { email: { status: "sent" } } } as Answer,
		remove = { status: 200, json: {} } as Answer,
	} = {},
) {
	const seen = {
		calls: [] as string[],
		bodies: [] as unknown[],
		auth: [] as string[],
	};
	const answer = (
		route: Parameters<Parameters<Page["route"]>[1]>[0],
		a: Answer,
	) => route.fulfill({ status: a.status, headers: cors, json: a.json ?? {} });
	await page.route(`${api}**`, (route) => {
		const req = route.request();
		if (req.method() === "OPTIONS") {
			return route.fulfill({ status: 204, headers: cors });
		}
		const path = new URL(req.url()).pathname;
		seen.calls.push(`${req.method()} ${path}`);
		seen.auth.push(req.headers().authorization ?? "");
		if (req.postData()) seen.bodies.push(req.postDataJSON());
		if (req.method() === "GET") return answer(route, list);
		if (req.method() === "POST" && path.endsWith("/invite")) {
			return answer(route, invite);
		}
		if (req.method() === "POST") {
			const body = req.postDataJSON();
			return answer(
				route,
				create ?? {
					status: 201,
					json: {
						admin: {
							id: "auth0|new",
							name: body.name,
							email: body.email,
							status: "invited",
							lastLogin: null,
						},
						email: { status: "sent" },
					},
				},
			);
		}
		if (req.method() === "DELETE") return answer(route, remove);
		if (req.method() === "PUT") {
			const id = decodeURIComponent(path.split("/").pop() ?? "");
			const admin = [ann, rob, cat].find((a) => a.id === id);
			return answer(route, { status: 200, json: { admin } });
		}
		return route.fulfill({ status: 404, headers: cors });
	});
	return seen;
}

const admins = (page: Page) => page.getByRole("list", { name: "Admins" });
const rowOf = (page: Page, name: string) =>
	admins(page).getByRole("listitem").filter({ hasText: name });
const heading = (page: Page) =>
	page.getByRole("heading", { level: 1, name: "Admins" });

async function openAdmins(page: Page) {
	await page.goto("/admin");
	await page.getByRole("banner").getByRole("link", { name: "Admins" }).click();
	await expect(heading(page)).toBeVisible();
	await expect(admins(page).getByRole("listitem")).not.toHaveCount(0);
}

test("the Admins link is hidden from someone who can't manage admins", async ({
	page,
}) => {
	await stubAuth0(page);
	await stubPosts(page);
	await page.goto("/admin");
	await expect(
		page.getByRole("heading", { level: 1, name: "News posts" }),
	).toBeVisible();
	await expect(
		page.getByRole("banner").getByRole("link", { name: "Help" }),
	).toBeVisible();
	await expect(
		page.getByRole("banner").getByRole("link", { name: "Admins" }),
	).toBeHidden();
});

test("the Admins link opens the admins in place, with how each is getting on", async ({
	page,
}) => {
	const auth = await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	const seen = await stubAdmins(page);
	await openAdmins(page);

	expect(new URL(page.url()).search).toBe("?admins");
	await expect(page).toHaveTitle("Admins · Club admin");
	await expect(heading(page)).toBeFocused();
	await expect(
		page.getByText("Admins can write and publish news on the website. Invite"),
	).toBeVisible();
	await expect(admins(page).getByRole("listitem")).toHaveCount(3);
	await expect(rowOf(page, "Ann Jones")).toContainText("ann@example.org");
	await expect(rowOf(page, "Ann Jones")).toContainText(
		"Invited, hasn't signed in yet",
	);
	await expect(rowOf(page, "Rob Smith")).toContainText(
		"Last signed in on 18 April",
	);
	await expect(rowOf(page, "Cat Moss")).toContainText(
		"Last signed in on 1 February 2025",
	);
	// Only an invitation can be sent again.
	await expect(
		rowOf(page, "Ann Jones").getByRole("button", {
			name: "Send the email again",
		}),
	).toBeVisible();
	await expect(
		rowOf(page, "Cat Moss").getByRole("button", {
			name: "Send the email again",
		}),
	).toBeHidden();
	expect(seen.auth[0]).toMatch(/^Bearer ey/);
	expect(auth.calls.filter((c) => c === "GET /authorize")).toHaveLength(1);

	// Back to the posts, still without a reload.
	await page.getByRole("link", { name: "All posts" }).click();
	await expect(
		page.getByRole("heading", { level: 1, name: "News posts" }),
	).toBeVisible();
	expect(auth.calls.filter((c) => c === "GET /authorize")).toHaveLength(1);
});

test("you can't remove your own access", async ({ page }) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	await stubAdmins(page);
	await openAdmins(page);

	const mine = rowOf(page, "Rob Smith");
	await expect(mine.getByText("You", { exact: true })).toBeVisible();
	await expect(
		rowOf(page, "Cat Moss").getByText("You", { exact: true }),
	).toBeHidden();
	await expect(
		mine.getByRole("button", { name: "Remove access" }),
	).toBeHidden();
	await expect(
		rowOf(page, "Cat Moss").getByRole("button", { name: "Remove access" }),
	).toBeVisible();
	await expect(
		admins(page).getByRole("button", { name: "Remove access" }),
	).toHaveCount(2);
});

test("inviting someone emails them and adds them to the list", async ({
	page,
}) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	const seen = await stubAdmins(page);
	await openAdmins(page);

	await page.getByLabel("Name").fill("Dee Brown");
	await page.getByLabel("Email address").fill("dee@example.org");
	await page.getByRole("button", { name: "Send invitation" }).click();

	await expect(
		page.getByRole("status").filter({ hasText: "emailed" }),
	).toHaveText(
		"We've emailed dee@example.org a link to choose a password. Ask them to check their spam folder if it hasn't arrived.",
	);
	expect(seen.calls).toContain("POST /api/admin/admins");
	expect(seen.bodies).toContainEqual({
		name: "Dee Brown",
		email: "dee@example.org",
	});
	await expect(admins(page).getByRole("listitem")).toHaveCount(4);
	// In name order, as the API sends them.
	await expect(admins(page).getByRole("listitem").nth(2)).toContainText(
		"Dee Brown",
	);
	await expect(rowOf(page, "Dee Brown")).toContainText(
		"Invited, hasn't signed in yet",
	);
	await expect(page.getByLabel("Name")).toHaveValue("");
	await expect(page.getByLabel("Email address")).toHaveValue("");
});

test("what's wrong with the name or email address shows beside it", async ({
	page,
}) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	await stubAdmins(page, {
		create: {
			status: 422,
			json: {
				error: "Check the details",
				fields: {
					name: "Enter their name",
					email: "Enter an email address like name@example.org",
				},
			},
		},
	});
	await openAdmins(page);

	await page.getByLabel("Email address").fill("dee");
	await page.getByRole("button", { name: "Send invitation" }).click();

	const name = page.getByLabel("Name");
	const email = page.getByLabel("Email address");
	await expect(name).toHaveAttribute("aria-invalid", "true");
	await expect(name).toHaveAccessibleDescription("Enter their name");
	await expect(email).toHaveAttribute("aria-invalid", "true");
	await expect(email).toHaveAccessibleDescription(
		"Enter an email address like name@example.org",
	);
	await expect(name).toBeFocused();
	await expect(admins(page).getByRole("listitem")).toHaveCount(3);

	// Typing clears the message.
	await name.fill("Dee Brown");
	await expect(name).not.toHaveAttribute("aria-invalid", "true");
	await expect(page.getByText("Enter their name")).toBeHidden();
});

test("when the email doesn't send, it can be sent again", async ({ page }) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	const seen = await stubAdmins(page, {
		create: {
			status: 201,
			json: {
				admin: {
					id: "auth0|dee",
					name: "Dee Brown",
					email: "dee@example.org",
					status: "invited",
					lastLogin: null,
				},
				email: { status: "failed" },
			},
		},
	});
	await openAdmins(page);

	await page.getByLabel("Name").fill("Dee Brown");
	await page.getByLabel("Email address").fill("dee@example.org");
	await page.getByRole("button", { name: "Send invitation" }).click();

	await expect(
		page.getByRole("status").filter({ hasText: "didn't send" }),
	).toContainText("Dee Brown can now be an admin, but the email didn't send.");
	const dee = rowOf(page, "Dee Brown");
	await dee.getByRole("button", { name: "Send the email again" }).click();
	await expect(dee).toContainText("Email sent again");
	expect(seen.calls).toContain("POST /api/admin/admins/auth0%7Cdee/invite");
});

test("Send the email again resends an invitation", async ({ page }) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	const seen = await stubAdmins(page);
	await openAdmins(page);

	await rowOf(page, "Ann Jones")
		.getByRole("button", { name: "Send the email again" })
		.click();
	await expect(rowOf(page, "Ann Jones")).toContainText("Email sent again");
	// The button stays, with focus, for another go.
	await expect(
		rowOf(page, "Ann Jones").getByRole("button", {
			name: "Send the email again",
		}),
	).toBeFocused();
	expect(seen.calls).toContain("POST /api/admin/admins/auth0%7Cann/invite");
});

test("a resend that fails says so", async ({ page }) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	await stubAdmins(page, {
		invite: {
			status: 502,
			json: { error: "We couldn't send the email. Try again in a minute." },
		},
	});
	await openAdmins(page);

	const resend = rowOf(page, "Ann Jones").getByRole("button", {
		name: "Send the email again",
	});
	await resend.click();
	await expect(page.getByRole("alert")).toContainText(
		"We couldn't send the email. Try again in a minute.",
	);
	await expect(resend).toBeFocused();
});

test("Remove access acts at once, and Undo gives it back", async ({ page }) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	const seen = await stubAdmins(page);
	await openAdmins(page);

	await rowOf(page, "Cat Moss")
		.getByRole("button", { name: "Remove access" })
		.click();
	await expect(rowOf(page, "Cat Moss")).toHaveCount(0);
	expect(seen.calls).toContain("DELETE /api/admin/admins/auth0%7Ccat");

	const toast = page.getByRole("status").filter({ hasText: "Access removed" });
	await toast.getByRole("button", { name: "Undo" }).click();
	await expect(rowOf(page, "Cat Moss")).toHaveCount(1);
	await expect(admins(page).getByRole("listitem").nth(1)).toContainText(
		"Cat Moss",
	);
	expect(seen.calls.slice(-2)).toEqual([
		"DELETE /api/admin/admins/auth0%7Ccat",
		"PUT /api/admin/admins/auth0%7Ccat",
	]);
});

test("a refused removal keeps the row and says why", async ({ page }) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	await stubAdmins(page, {
		remove: {
			status: 409,
			json: { error: "You can't remove your own access" },
		},
	});
	await openAdmins(page);

	await rowOf(page, "Cat Moss")
		.getByRole("button", { name: "Remove access" })
		.click();
	await expect(page.getByRole("alert")).toContainText(
		"You can't remove your own access",
	);
	await expect(rowOf(page, "Cat Moss")).toHaveCount(1);
});

for (const [status, error] of [
	[409, "They're already an admin"],
	[502, "We couldn't send the invitation. Try again in a minute."],
	[503, "Inviting admins isn't set up yet"],
] as const) {
	test(`an invitation refused with ${status} shows the API's message`, async ({
		page,
	}) => {
		await stubAuth0(page, { permissions: managers });
		await stubPosts(page);
		await stubAdmins(page, { create: { status, json: { error } } });
		await openAdmins(page);

		await page.getByLabel("Name").fill("Ann Jones");
		await page.getByLabel("Email address").fill("ann@example.org");
		await page.getByRole("button", { name: "Send invitation" }).click();
		await expect(page.getByRole("alert")).toContainText(error);
		await expect(admins(page).getByRole("listitem")).toHaveCount(3);
		// What they typed stays, to correct or try again.
		await expect(page.getByLabel("Email address")).toHaveValue(
			"ann@example.org",
		);
	});
}

test("without permission, the admins view says so", async ({ page }) => {
	await stubAuth0(page);
	await stubPosts(page);
	await stubAdmins(page, { list: { status: 403 } });
	await page.goto("/admin?admins");

	await expect(heading(page)).toBeVisible();
	await expect(page.getByRole("alert")).toHaveText(
		"You don't have permission to manage admins.",
	);
	await expect(page.getByLabel("Name")).toBeHidden();
	await expect(admins(page)).toBeHidden();
});

test("before the server is set up, the admins view says so", async ({
	page,
}) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	await stubAdmins(page, {
		list: { status: 503, json: { error: "Inviting admins isn't set up yet" } },
	});
	await page.goto("/admin?admins");

	await expect(page.getByRole("alert")).toHaveText(
		"Inviting admins isn't set up yet.",
	);
	await expect(page.getByLabel("Name")).toBeHidden();
});

test("the guide explains inviting and removing admins", async ({ page }) => {
	await stubAuth0(page, { permissions: managers });
	await stubPosts(page);
	await stubAdmins(page);
	await openAdmins(page);
	await page.getByRole("link", { name: "How to invite an admin" }).click();

	const guide = page.getByRole("heading", {
		level: 2,
		name: "Inviting an admin",
	});
	await expect(guide).toBeFocused();
	expect(new URL(page.url()).search).toBe("?help=admins");
	const region = page.getByRole("region", { name: "Inviting an admin" });
	for (const step of [
		"permission to manage admins",
		"Send invitation",
		"spam",
		"passkey",
		"Remove access",
		"Undo",
	]) {
		await expect(region).toContainText(step);
	}
});
