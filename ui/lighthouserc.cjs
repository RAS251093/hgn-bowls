// Lighthouse CI: the performance and accessibility budget for public pages.
// It serves the built site itself, runs each page three times on the
// default (phone) profile and judges the median run, so one slow run on a
// busy CI machine doesn't fail the build.
//
// Run with `npm run lighthouse` after a build. @lhci/cli is fetched at a
// pinned version rather than installed: its dependency tree carries known
// vulnerabilities in code paths this use never reaches (zip extraction,
// FTP, proxies), and keeping it out of package-lock.json keeps the
// project's own audit clean.
module.exports = {
	ci: {
		collect: {
			staticDistDir: "./dist",
			url: ["http://localhost/", "http://localhost/whats-on/"],
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
