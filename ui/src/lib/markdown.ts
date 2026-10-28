// Turns a post body (Markdown from the admin editor) into safe HTML at build
// time. Raw HTML in the Markdown is dropped rather than passed through, and
// the result is sanitised with GitHub's allowlist, so a post can't add
// scripts, styles, frames or javascript: links.
import type { Element, Root } from "hast";
import rehypeSanitize from "rehype-sanitize";
import rehypeStringify from "rehype-stringify";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import { unified } from "unified";

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

const processor = unified()
	.use(remarkParse)
	.use(remarkRehype) // no allowDangerousHtml: raw HTML never reaches the output
	.use(rehypeSanitize)
	.use(noSecondH1)
	.use(rehypeStringify);

export async function renderPost(markdown: string): Promise<string> {
	return String(await processor.process(markdown));
}
