import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { renderPost } from "./markdown.ts";
import { youtubeVideo } from "./youtube.ts";

describe("youtubeVideo", () => {
	const tests: { url: string; want: { id: string; start: number } | null }[] = [
		{
			url: "https://youtu.be/LegryDpTHXo",
			want: { id: "LegryDpTHXo", start: 0 },
		},
		{
			url: "https://www.youtube.com/watch?v=LegryDpTHXo",
			want: { id: "LegryDpTHXo", start: 0 },
		},
		{
			url: "https://m.youtube.com/watch?v=LegryDpTHXo&t=90",
			want: { id: "LegryDpTHXo", start: 90 },
		},
		{
			url: "https://youtu.be/LegryDpTHXo?t=1m30s",
			want: { id: "LegryDpTHXo", start: 90 },
		},
		{
			url: "https://www.youtube.com/shorts/LegryDpTHXo",
			want: { id: "LegryDpTHXo", start: 0 },
		},
		{
			url: "https://www.youtube.com/embed/LegryDpTHXo",
			want: { id: "LegryDpTHXo", start: 0 },
		},
		{
			url: "https://www.youtube-nocookie.com/embed/LegryDpTHXo",
			want: { id: "LegryDpTHXo", start: 0 },
		},
		{ url: "https://youtu.be/short", want: null },
		{ url: 'https://youtu.be/LegryDpTHXo"onload=x', want: null },
		{ url: "https://evil.example/watch?v=LegryDpTHXo", want: null },
		{ url: "https://www.youtube.com/@club", want: null },
		{ url: "not a url", want: null },
	];
	for (const t of tests) {
		test(t.url, () => assert.deepEqual(youtubeVideo(t.url), t.want));
	}
});

// A fake YouTube: oEmbed answers with a title and thumbnail, the thumbnail is a few bytes.
function fakeYouTube(fail = false) {
	const asked: string[] = [];
	const get = async (url: string | URL | Request) => {
		const u = String(url);
		asked.push(u);
		if (fail) throw new Error("offline");
		if (u.startsWith("https://www.youtube.com/oembed")) {
			return Response.json({
				title: 'Club "final" <2026>',
				thumbnail_url: "https://i.ytimg.com/vi/LegryDpTHXo/hqdefault.jpg",
			});
		}
		return new Response(new Uint8Array([0xff, 0xd8, 0xff]), {
			headers: { "Content-Type": "image/jpeg" },
		});
	};
	return { get, asked };
}

describe("videos in posts", () => {
	test("a YouTube link on its own line becomes a video that loads only when played", async () => {
		const yt = fakeYouTube();
		const html = await renderPost(
			"Highlights:\n\nhttps://youtu.be/LegryDpTHXo?t=90",
			{ fetch: yt.get },
		);
		assert.match(
			html,
			/<figure[^>]*data-youtube="LegryDpTHXo"[^>]*data-start="90"/,
		);
		assert.match(
			html,
			/<a[^>]*href="https:\/\/www\.youtube\.com\/watch\?v=LegryDpTHXo&#x26;t=90"/,
		);
		// A quote in the title can't end the attribute; < inside quotes is harmless.
		assert.match(
			html,
			/aria-label="Play video: Club &#x22;final&#x22; <2026>"/,
		);
		assert.match(html, /<img[^>]*src="data:image\/jpeg;base64,/);
		assert.match(
			html,
			/<figcaption[^>]*>Club "final" &#x3C;2026><\/figcaption>/,
		);
		assert.doesNotMatch(html, /<iframe|i\.ytimg\.com/);
	});

	test("a link with its own text uses that text when YouTube gives no title", async () => {
		const yt = fakeYouTube(true);
		const html = await renderPost("[The final](https://youtu.be/LegryDpTHXo)", {
			fetch: yt.get,
		});
		assert.match(html, /data-youtube="LegryDpTHXo"/);
		assert.match(html, /Play video: The final/);
		assert.doesNotMatch(html, /<img/);
	});

	test("with YouTube unreachable, the video still plays, without a thumbnail", async () => {
		const html = await renderPost("https://youtu.be/LegryDpTHXo", {
			fetch: fakeYouTube(true).get,
		});
		assert.match(html, /data-youtube="LegryDpTHXo"/);
		assert.match(html, /Play video: Watch on YouTube/);
	});

	test("without a fetcher, nothing is fetched", async () => {
		const html = await renderPost("https://youtu.be/LegryDpTHXo");
		assert.match(html, /data-youtube="LegryDpTHXo"/);
	});

	test("an embed copied from YouTube (as Hygraph stored them) becomes the same video", async () => {
		const iframe =
			'<iframe width="560" height="315" frameborder="0" allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture" allowfullscreen="" title="https://www.youtube.com/embed/LegryDpTHXo?si=epLayir2RQ9GiSgG" src="https://www.youtube.com/embed/LegryDpTHXo?si=epLayir2RQ9GiSgG"></iframe>';
		const html = await renderPost(`Highlights:\n\n${iframe}\n\nWell played.`);
		assert.match(html, /data-youtube="LegryDpTHXo"/);
		assert.doesNotMatch(html, /<iframe|si=epLayir/);
		assert.match(html, /<p>Well played\.<\/p>/);
	});

	test("an iframe from anywhere else is still dropped", async () => {
		const html = await renderPost(
			'<iframe src="https://evil.example/embed/LegryDpTHXo"></iframe>',
		);
		assert.doesNotMatch(html, /data-youtube|<iframe|evil/);
	});

	test("a YouTube link inside a sentence stays a link", async () => {
		const yt = fakeYouTube();
		const html = await renderPost(
			"Watch https://youtu.be/LegryDpTHXo tonight.",
			{ fetch: yt.get },
		);
		assert.doesNotMatch(html, /data-youtube/);
		assert.match(html, /<a href="https:\/\/youtu\.be\/LegryDpTHXo">/);
		assert.deepEqual(yt.asked, []);
	});

	test("other links on their own line stay links", async () => {
		const html = await renderPost("https://example.org/fixtures", {
			fetch: fakeYouTube().get,
		});
		assert.doesNotMatch(html, /data-youtube/);
	});
});
