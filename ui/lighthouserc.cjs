// Lighthouse CI: the performance and accessibility budget for public pages.
// It serves the built site itself, runs each page three times on the
// default (phone) profile and judges the median run, so one slow run on a
// busy CI machine doesn't fail the build.
//
// Run with `npm run lighthouse` after a build. @lhci/cli lives in
// tools/lighthouse with its own lockfile, so every package it pulls in is
// pinned (fetching it with npx picked up whatever was newest that minute).
// Keeping it out of the site's package-lock.json keeps the site's audit
// clean: its tree carries known vulnerabilities in code paths this use
// never reaches (zip extraction, FTP, a localhost-only server).
module.exports = {
	ci: {
		collect: {
			staticDistDir: "./dist",
			url: [
				"http://localhost/",
				"http://localhost/whats-on/",
				"http://localhost/news/",
				"http://localhost/news/sample-opening-day/",
			],
			numberOfRuns: 3,
		},
		assert: {
			assertions: {
				"categories:performance": [
					"error",
					{ minScore: 0.95, aggregationMethod: "median-run" },
				],
				"categories:accessibility": [
					"error",
					{ minScore: 1, aggregationMethod: "median-run" },
				],
			},
		},
		upload: {
			// Reports stay with the CI run, not on Google's public report storage.
			target: "filesystem",
			outputDir: ".lighthouseci/reports",
		},
	},
};
