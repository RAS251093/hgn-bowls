// Turns a post body (Markdown from the admin editor) into safe HTML at build
// time, with GitHub's Markdown extensions. Raw HTML in the Markdown is dropped rather than passed through, and
// the result is sanitised with GitHub's allowlist, so a post can't add
// scripts, styles, frames or javascript: links.
import type { Element, Root } from "hast";
import rehypeSanitize from "rehype-sanitize";
import rehypeStringify from "rehype-stringify";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import { unified } from "unified";
import { rehypeYoutube, remarkYoutubeIframes } from "./youtube.ts";

// The post title is the page's only h1, so a "# heading" in the body
// becomes an h2, the same as the editor's subheadings.
function noSecondH1() {
	return (tree: Root) => {
		const visit = (node: Root | Element) => {
			for (const child of node.children) {
				if (child.type !== "element") continue;
				if (child.tagName === "h1") child.tagName = "h2";
				visit(child);
			}
		};
		visit(tree);
	};
}

export interface RenderOptions {
	/** Used at build time to fetch video titles and thumbnails. Leave unset to fetch nothing. */
	fetch?: typeof fetch;
}

export async function renderPost(
	markdown: string,
	{ fetch: get }: RenderOptions = {},
): Promise<string> {
	const processor = unified()
		.use(remarkParse)
		.use(remarkGfm) // pasted addresses become links; tables and strikethrough
		.use(remarkYoutubeIframes) // a YouTube embed iframe becomes a link, before raw HTML is dropped
		.use(remarkRehype) // no allowDangerousHtml: raw HTML never reaches the output
		.use(rehypeSanitize)
		.use(noSecondH1)
		.use(rehypeYoutube, { fetch: get }) // after sanitising: the player markup is ours
		.use(rehypeStringify);
	return String(await processor.process(markdown));
}
