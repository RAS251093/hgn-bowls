import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { deployState } from "./deploys.ts";

const build = (id: string, status: "building" | "live" | "failed") => ({
	id,
	status,
});

describe("deployState", () => {
	test("waits until a build newer than the one before publishing appears", () => {
		// Right after publishing, the newest build is still the old one, which
		// is live; reading that as done would say Live too early.
		assert.equal(deployState("old", build("old", "live")), "waiting");
		assert.equal(deployState("old", null), "waiting");
	});

	test("then follows the new build", () => {
		assert.equal(deployState("old", build("new", "building")), "building");
		assert.equal(deployState("old", build("new", "live")), "live");
		assert.equal(deployState("old", build("new", "failed")), "failed");
	});

	test("with no build before (a brand-new project), any build is the new one", () => {
		assert.equal(deployState(null, build("first", "live")), "live");
	});
});
