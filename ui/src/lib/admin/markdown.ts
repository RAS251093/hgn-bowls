// What the post editor can hold, and its conversion to and from the
// Markdown the API stores. Admins only ever see rich text: subheadings,
// bold, italic, links, lists and quotes, which is everything the site's
// renderer (src/lib/markdown.ts) shows for them.
import type { AnyExtension, JSONContent } from "@tiptap/core";
import { MarkdownManager } from "@tiptap/markdown";
import StarterKit from "@tiptap/starter-kit";
import { youtubeVideo } from "../youtube.ts";

export function editorExtensions(): AnyExtension[] {
	return [
		StarterKit.configure({
			heading: { levels: [2, 3] }, // the post title is the page's h1
			code: false,
			codeBlock: false,
			strike: false,
			underline: false, // Markdown has no underline
			horizontalRule: false,
			trailingNode: false, // it would save an empty paragraph at the end
			link: {
				openOnClick: false,
				autolink: true,
				protocols: ["mailto"],
				defaultProtocol: "https",
			},
		}),
	];
}

const manager = new MarkdownManager({ extensions: editorExtensions() });

// The old site's YouTube embeds, on a line of their own.
const iframeLine =
	/^[ \t]*<iframe\b[^>]*\ssrc="([^"]+)"[^>]*>\s*<\/iframe>[ \t]*$/gim;

/**
 * Tidies stored Markdown into what the editor can show: an embedded
 * YouTube player becomes its link (which the site turns back into a
 * player), headings fit the editor's two levels, and any other raw HTML is
 * dropped, as the site drops it, rather than shown as text.
 */
function forEditor(markdown: string): string {
	const tidied = markdown
		.replace(iframeLine, (line, src: string) => {
			const video = youtubeVideo(src);
			return video ? `https://youtu.be/${video.id}` : line;
		})
		.replace(/^#[ \t]/gm, "## ")
		.replace(/^#{4,6}[ \t]/gm, "### ");
	return manager.instance
		.lexer(tidied)
		.filter((token) => token.type !== "html")
		.map((token) => token.raw)
		.join("");
}

export function fromMarkdown(markdown: string): JSONContent {
	return manager.parse(forEditor(markdown));
}

// Paragraphs with nothing in them: extra presses of Enter. Markdown can't
// hold them except as &nbsp;, which would show as blank space on the site.
const isEmptyParagraph = (node: JSONContent) =>
	node.type === "paragraph" && !node.content?.length;

export function toMarkdown(doc: JSONContent): string {
	const content = (doc.content ?? []).filter((n) => !isEmptyParagraph(n));
	return manager.serialize({ ...doc, content }).trim();
}
