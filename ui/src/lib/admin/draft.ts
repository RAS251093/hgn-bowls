// A post as the editor saves it (the API's Draft), and the same checks the
// API makes, so problems show beside the field as the admin types.

export interface Draft {
	title: string;
	description: string;
	bodyMd: string;
	/** Carried through untouched until the editor handles covers. */
	coverImageKey: string | null;
	coverAlt: string | null;
}

/** Field name (as the API names it) to what to do about it. */
export type FieldErrors = Partial<Record<keyof Draft, string>>;

// The API's limits (api/posts/post.go), counted in characters.
export const limits = { title: 120, description: 300 };

const length = (s: string) => [...s].length;

export function validateDraft(d: Draft): FieldErrors {
	const errors: FieldErrors = {};
	const title = d.title.trim();
	if (!title) errors.title = "Add a headline";
	else if (length(title) > limits.title)
		errors.title = `Keep the headline to ${limits.title} characters or fewer`;
	if (length(d.description.trim()) > limits.description)
		errors.description = `Keep the description to ${limits.description} characters or fewer`;
	return errors;
}
