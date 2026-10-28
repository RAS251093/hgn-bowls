// YouTube videos in posts. A YouTube link on its own line (or a YouTube
// embed iframe, as Hygraph stored them) becomes a thumbnail with a play
// button. The title and thumbnail are fetched once at build time and the
// thumbnail is inlined, so reading a post makes no request to YouTube; the
// player (youtube-nocookie.com) loads only when someone presses play.
import type { Element, ElementContent, Root as HastRoot } from "hast";
import type { Html, Root as MdRoot, Paragraph } from "mdast";

export interface Video {
	id: string;
	/** Start time in seconds. */
	start: number;
}

const idPattern = /^[\w-]{11}$/;

/** Reads a YouTube video address. Anything else, including channel pages, gives null. */
export function youtubeVideo(href: string): Video | null {
	let url: URL;
	try {
		url = new URL(href);
	} catch {
		return null;
	}
	if (url.protocol !== "https:") return null;
	const host = url.hostname.replace(/^(www|m)\./, "");
	const path = url.pathname.split("/").filter(Boolean);
	let id = "";
	if (host === "youtu.be") {
		id = path[0] ?? "";
	} else if (host === "youtube.com" || host === "youtube-nocookie.com") {
		if (path[0] === "watch") id = url.searchParams.get("v") ?? "";
		else if (["embed", "shorts", "live"].includes(path[0] ?? ""))
			id = path[1] ?? "";
	}
	if (!idPattern.test(id) || path.length > 2) return null;
	return {
		id,
		start: seconds(
			url.searchParams.get("t") ?? url.searchParams.get("start") ?? "",
		),
	};
}

// "90", "90s" or "1m30s".
function seconds(t: string): number {
	const m = /^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s?)?$/.exec(t);
	if (!t || !m) return 0;
	return Number(m[1] ?? 0) * 3600 + Number(m[2] ?? 0) * 60 + Number(m[3] ?? 0);
}

const iframe = /^\s*<iframe\b[^>]*\ssrc="([^"]+)"[^>]*>\s*<\/iframe>\s*$/i;

/**
 * Markdown step, before raw HTML is dropped: a block that is just a YouTube
 * embed iframe becomes a link to the video, which rehypeYoutube then turns
 * into a player. Every other piece of raw HTML is left to be dropped.
 */
export function remarkYoutubeIframes() {
	return (tree: MdRoot) => {
		tree.children = tree.children.map((node) => {
			if (node.type !== "html") return node;
			const src = iframe.exec((node as Html).value)?.[1];
			const video = src ? youtubeVideo(src) : null;
			if (!video) return node;
			const href = `https://youtu.be/${video.id}`;
			const link: Paragraph = {
				type: "paragraph",
				children: [
					{
						type: "link",
						url: href,
						children: [{ type: "text", value: href }],
					},
				],
			};
			return link;
		});
	};
}

interface Options {
	/** Fetches YouTube's oEmbed data and thumbnail. Without one, videos get no title or thumbnail. */
	fetch?: typeof fetch;
}

/** HTML step, after sanitising: a paragraph holding only a YouTube link becomes a player. */
export function rehypeYoutube({ fetch: get }: Options = {}) {
	return async (tree: HastRoot) => {
		const jobs: Promise<void>[] = [];
		tree.children.forEach((node, i) => {
			const link = onlyLink(node);
			const video = link
				? youtubeVideo(String(link.properties.href ?? ""))
				: null;
			if (!link || !video) return;
			const text = textOf(link);
			const ownText = text !== String(link.properties.href) ? text : "";
			jobs.push(
				details(video.id, get).then(({ title, thumbnail }) => {
					tree.children[i] = player(
						video,
						title || ownText || "Watch on YouTube",
						thumbnail,
					);
				}),
			);
		});
		await Promise.all(jobs);
	};
}

function onlyLink(node: HastRoot["children"][number]): Element | null {
	if (node.type !== "element" || node.tagName !== "p") return null;
	const kids = node.children.filter(
		(c) => !(c.type === "text" && !c.value.trim()),
	);
	const [only] = kids;
	return kids.length === 1 && only?.type === "element" && only.tagName === "a"
		? only
		: null;
}

function textOf(node: ElementContent): string {
	if (node.type === "text") return node.value;
	if (node.type === "element") return node.children.map(textOf).join("");
	return "";
}

const maxThumbnail = 200 * 1024;

async function details(
	id: string,
	get?: typeof fetch,
): Promise<{ title: string; thumbnail: string }> {
	if (!get) return { title: "", thumbnail: "" };
	try {
		const watch = `https://www.youtube.com/watch?v=${id}`;
		const res = await get(
			`https://www.youtube.com/oembed?format=json&url=${encodeURIComponent(watch)}`,
			{
				signal: AbortSignal.timeout(5000),
			},
		);
		if (!res.ok) return { title: "", thumbnail: "" };
		const info = (await res.json()) as {
			title?: string;
			thumbnail_url?: string;
		};
		const title = info.title ?? "";
		if (!info.thumbnail_url?.startsWith("https://i.ytimg.com/"))
			return { title, thumbnail: "" };
		const img = await get(info.thumbnail_url, {
			signal: AbortSignal.timeout(5000),
		});
		const type = img.headers.get("Content-Type") ?? "";
		const bytes = new Uint8Array(await img.arrayBuffer());
		if (
			!img.ok ||
			!/^image\/(jpeg|png|webp)$/.test(type) ||
			bytes.length > maxThumbnail
		)
			return { title, thumbnail: "" };
		return {
			title,
			thumbnail: `data:${type};base64,${Buffer.from(bytes).toString("base64")}`,
		};
	} catch {
		// YouTube unreachable at build time: the video still plays, without a thumbnail.
		return { title: "", thumbnail: "" };
	}
}

const el = (
	tagName: string,
	properties: Element["properties"],
	children: ElementContent[] = [],
): Element => ({
	type: "element",
	tagName,
	properties,
	children,
});

// Lucide "play", outlined.
const playIcon = el(
	"svg",
	{
		width: 20,
		height: 20,
		viewBox: "0 0 24 24",
		fill: "none",
		stroke: "currentColor",
		strokeWidth: "2",
		strokeLinecap: "round",
		strokeLinejoin: "round",
		ariaHidden: "true",
	},
	[el("polygon", { points: "6 3 20 12 6 21 6 3" })],
);

function player(video: Video, title: string, thumbnail: string): Element {
	const watch = `https://www.youtube.com/watch?v=${video.id}${video.start ? `&t=${video.start}` : ""}`;
	return el(
		"figure",
		{
			className: ["flex", "flex-col", "gap-3"],
			dataYoutube: video.id,
			dataStart: String(video.start),
			dataTitle: title,
		},
		[
			el(
				"a",
				{
					href: watch,
					ariaLabel: `Play video: ${title}`,
					className: [
						"group",
						"relative",
						"block",
						"aspect-video",
						"overflow-hidden",
						"rounded-lg",
						"bg-primary",
						"no-underline",
					],
				},
				[
					...(thumbnail
						? [
								el("img", {
									src: thumbnail,
									alt: "",
									className: [
										"absolute",
										"inset-0",
										"h-full",
										"w-full",
										"object-cover",
									],
								}),
							]
						: []),
					el(
						"span",
						{
							className: [
								"absolute",
								"inset-0",
								"m-auto",
								"flex",
								"h-12",
								"w-fit",
								"items-center",
								"gap-2",
								"rounded-md",
								"bg-accent",
								"px-5",
								"text-label",
								"text-accent-foreground",
								"transition-colors",
								"duration-120",
								"group-hover:bg-card",
							],
						},
						[playIcon, { type: "text", value: "Play video" }],
					),
				],
			),
			el(
				"figcaption",
				{ className: ["text-body-sm", "text-muted-foreground"] },
				[{ type: "text", value: title }],
			),
		],
	);
}
