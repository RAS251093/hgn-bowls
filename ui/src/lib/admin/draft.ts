// A post as the editor saves it (the API's Draft), and the same checks the
// API makes, so problems show beside the field as the admin types.

export interface Draft {
	title: string;
	description: string;
	bodyMd: string;
	/** The cover's object name in Cloud Storage, or null for the club's default photo. */
	coverImageKey: string | null;
	coverAlt: string | null;
}

/** Field name (as the API names it) to what to do about it. */
export type FieldErrors = Partial<Record<keyof Draft, string>>;

// The API's limits (api/posts/post.go), counted in characters.
export const limits = { title: 120, description: 300, coverAlt: 300 };

const length = (s: string) => [...s].length;

export function validateDraft(d: Draft): FieldErrors {
	const errors: FieldErrors = {};
	const title = d.title.trim();
	if (!title) errors.title = "Add a headline";
	else if (length(title) > limits.title)
		errors.title = `Keep the headline to ${limits.title} characters or fewer`;
	if (length(d.description.trim()) > limits.description)
		errors.description = `Keep the description to ${limits.description} characters or fewer`;
	if (d.coverImageKey) {
		const alt = (d.coverAlt ?? "").trim();
		if (!alt)
			errors.coverAlt = "Describe the photo for people who can't see it";
		else if (length(alt) > limits.coverAlt)
			errors.coverAlt = `Keep the description to ${limits.coverAlt} characters or fewer`;
	}
	return errors;
}
