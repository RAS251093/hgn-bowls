import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { fitWithin } from "./shrink.ts";

describe("fitWithin", () => {
	test("scales the longest side down to the limit, keeping the shape", () => {
		assert.deepEqual(fitWithin(4032, 3024, 2000), {
			width: 2000,
			height: 1500,
		});
		assert.deepEqual(fitWithin(3024, 4032, 2000), {
			width: 1500,
			height: 2000,
		});
		assert.deepEqual(fitWithin(3130, 1565, 2000), {
			width: 2000,
			height: 1000,
		});
	});

	test("leaves photos already within the limit alone", () => {
		assert.deepEqual(fitWithin(1600, 900, 2000), { width: 1600, height: 900 });
		assert.deepEqual(fitWithin(2000, 2000, 2000), {
			width: 2000,
			height: 2000,
		});
	});

	test("never rounds a side down to nothing", () => {
		assert.deepEqual(fitWithin(10000, 1, 2000), { width: 2000, height: 1 });
	});
});
