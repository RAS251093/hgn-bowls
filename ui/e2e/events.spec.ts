import { expect, type Page, test } from "@playwright/test";

const events = [
	{
		id: "a",
		title: "Green and clubhouse maintenance",
		start: "2026-11-02T09:30:00Z",
		end: "2026-11-02T10:30:00Z",
		allDay: false,
		location: "Main green",
		description: "Bring gloves.\nTea afterwards.",
	},
	{
		id: "b",
		title: "Christmas social",
		start: "2026-12-18",
		end: "2026-12-19",
		allDay: true,
		location: "Clubhouse",
		description: "",
	},
	{
		id: "c",
		title: "New year bowls",
		start: "2027-01-02T13:00:00Z",
		end: "",
		allDay: false,
		location: "",
		description: "",
	},
	{
		id: "d",
		title: "Committee meeting",
		start: "2027-01-12T19:00:00Z",
		end: "2027-01-12T20:30:00Z",
		allDay: false,
		location: "Clubhouse",
		description: "",
	},
	{
		id: "e",
		title: "Coaching for beginners",
		start: "2027-01-16T10:00:00Z",
		end: "2027-01-16T12:00:00Z",
		allDay: false,
		location: "Main green",
		description: "",
	},
];

// Pins the page's clock so month headings don't depend on when tests run.
async function at(page: Page, iso: string) {
	await page.clock.setFixedTime(new Date(iso));
}

function stub(page: Page, status: number, body: unknown) {
	return page.route("https://api.test/api/events*", (route) =>
		route.fulfill({
			status,
			contentType: "application/json",
			body: JSON.stringify(body),
		}),
	);
}

test.describe("What's on", () => {
	test.beforeEach(({ page }) => at(page, "2026-11-01T10:00:00Z"));

	test("lists upcoming events by month", async ({ page }) => {
		await stub(page, 200, { events, stale: false });
		await page.goto("/whats-on");

		await expect(
			page.getByRole("heading", { level: 1, name: "What's on" }),
		).toBeVisible();
		const months = page.getByRole("heading", { level: 2 });
		await expect(months).toHaveText(["November", "December", "January 2027"]);

		const first = page
			.getByRole("listitem")
			.filter({ hasText: "Green and clubhouse maintenance" });
		await expect(first.getByRole("heading", { level: 3 })).toHaveText(
			"Monday 2 November: Green and clubhouse maintenance",
		);
		await expect(first).toContainText("9:30am to 10:30am · Main green");
		await expect(first).toContainText("Bring gloves.");
		await expect(
			page.getByRole("listitem").filter({ hasText: "Christmas social" }),
		).toContainText("All day · Clubhouse");
		await expect(page.getByText("These may be out of date")).toBeHidden();
	});

	test("asks the API for up to 50 events", async ({ page }) => {
		let asked = "";
		await page.route("https://api.test/api/events*", (route) => {
			asked = new URL(route.request().url()).searchParams.get("limit") ?? "";
			return route.fulfill({ json: { events: [], stale: false } });
		});
		await page.goto("/whats-on");
		await expect(page.getByText("Nothing on the calendar yet")).toBeVisible();
		expect(asked).toBe("50");
	});

	test("says when the events may be out of date", async ({ page }) => {
		await stub(page, 200, { events, stale: true });
		await page.goto("/whats-on");
		await expect(page.getByText("These may be out of date")).toBeVisible();
	});

	test("an empty calendar points to Google Calendar", async ({ page }) => {
		await stub(page, 200, { events: [], stale: false });
		await page.goto("/whats-on");
		await expect(page.getByText("Nothing on the calendar yet")).toBeVisible();
		await expect(
			page.getByRole("link", { name: "See the calendar in Google" }),
		).toHaveAttribute("href", /calendar\.google\.com\/calendar\/embed\?src=/);
	});

	test("when the API fails, the Google Calendar link takes over", async ({
		page,
	}) => {
		await stub(page, 503, { error: "events are unavailable right now" });
		await page.goto("/whats-on");
		await expect(
			page.getByText("We can't show the events right now"),
		).toBeVisible();
		await expect(
			page.getByRole("link", { name: "See the calendar in Google" }),
		).toBeVisible();
		await expect(page.getByRole("listitem")).toHaveCount(0);
	});

	test("when the API can't be reached, the Google Calendar link takes over", async ({
		page,
	}) => {
		await page.route("https://api.test/api/events*", (route) => route.abort());
		await page.goto("/whats-on");
		await expect(
			page.getByText("We can't show the events right now"),
		).toBeVisible();
	});

	test("offers the calendar to subscribe to and to open in Google", async ({
		page,
	}) => {
		await stub(page, 200, { events: [], stale: false });
		await page.goto("/whats-on");
		await expect(
			page.getByRole("link", { name: "Add to my calendar" }),
		).toHaveAttribute(
			"href",
			/^webcal:\/\/calendar\.google\.com\/calendar\/ical\/.+\/public\/basic\.ics$/,
		);
		await expect(
			page.getByRole("link", { name: "Open in Google Calendar" }),
		).toHaveAttribute("href", /calendar\.google\.com\/calendar\/embed\?src=/);
	});

	test("event text is shown as text, never as HTML", async ({ page }) => {
		await stub(page, 200, {
			events: [
				{
					...events[0],
					title: '<img src=x onerror="window.hacked=1">Open day',
				},
			],
			stale: false,
		});
		await page.goto("/whats-on");
		await expect(
			page.getByText('<img src=x onerror="window.hacked=1">Open day'),
		).toBeVisible();
		expect(await page.evaluate(() => "hacked" in window)).toBe(false);
	});
});

test.describe("Home", () => {
	test.beforeEach(({ page }) => at(page, "2026-11-01T10:00:00Z"));

	test("shows the next four events and links to the rest", async ({ page }) => {
		let asked = "";
		await page.route("https://api.test/api/events*", (route) => {
			asked = new URL(route.request().url()).searchParams.get("limit") ?? "";
			return route.fulfill({
				json: { events: events.slice(0, 4), stale: false },
			});
		});
		await page.goto("/");
		const box = page.getByRole("region", { name: "What's on" });
		await expect(box.getByRole("listitem")).toHaveCount(4);
		expect(asked).toBe("4");
		await expect(
			box.getByRole("link", { name: "See all upcoming events" }),
		).toHaveAttribute("href", "/whats-on");
	});
});
