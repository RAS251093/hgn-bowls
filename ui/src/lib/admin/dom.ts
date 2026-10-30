// Small DOM helpers shared by the admin views.

/** The element matching selector; the page is built with it, so a miss is a bug. */
export function el<T extends Element = HTMLElement>(
	selector: string,
	root: ParentNode = document,
): T {
	const found = root.querySelector<T>(selector);
	if (!found) throw new Error(`admin: missing ${selector}`);
	return found;
}

/** A copy of a <template>'s content. */
export function template(selector: string): DocumentFragment {
	return el<HTMLTemplateElement>(`template${selector}`).content.cloneNode(
		true,
	) as DocumentFragment;
}

/** Thrown when the API says the admin lacks the club-admin role. */
export class Forbidden extends Error {
	constructor() {
		super("not a club admin");
		this.name = "Forbidden";
	}
}

/** A view on screen; leave() runs before another replaces it. */
export interface View {
	leave(): Promise<void>;
}
