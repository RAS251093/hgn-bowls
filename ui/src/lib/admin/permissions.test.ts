import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { permissionsOf } from "./permissions.ts";

const b64url = (o: unknown) =>
	Buffer.from(JSON.stringify(o)).toString("base64url");
const jwt = (claims: unknown) =>
	[b64url({ alg: "RS256", typ: "JWT" }), b64url(claims), "signature"].join(".");

describe("permissionsOf", () => {
	test("reads the permissions claim from an access token", () => {
		assert.deepEqual(
			permissionsOf(
				jwt({
					sub: "auth0|rob",
					permissions: ["manage:posts", "manage:admins"],
				}),
			),
			["manage:posts", "manage:admins"],
		);
	});

	test("copes with base64url characters and no padding", () => {
		// "~~~" and "???" encode to "-" and "_" in base64url.
		const token = jwt({ name: "~~~???", permissions: ["manage:admins"] });
		assert.match(token.split(".")[1], /[-_]/);
		assert.deepEqual(permissionsOf(token), ["manage:admins"]);
	});

	test("a token without permissions has none", () => {
		assert.deepEqual(permissionsOf(jwt({ sub: "auth0|rob" })), []);
		assert.deepEqual(permissionsOf(jwt({ permissions: "manage:admins" })), []);
		assert.deepEqual(permissionsOf(jwt(null)), []);
		assert.deepEqual(permissionsOf(jwt([1, 2])), []);
	});

	test("keeps only strings", () => {
		assert.deepEqual(
			permissionsOf(jwt({ permissions: ["manage:posts", 3, null] })),
			["manage:posts"],
		);
	});

	test("anything that isn't a JWT has none", () => {
		for (const token of [
			"",
			"stub-access-token",
			"a.b.c",
			"a.!!!.c",
			`x.${Buffer.from("not json").toString("base64url")}.y`,
		]) {
			assert.deepEqual(permissionsOf(token), [], token);
		}
	});
});
