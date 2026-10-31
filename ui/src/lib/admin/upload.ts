// Uploads a cover photo in two steps: the API hands out a signed address
// locked to one new object name, type and size, then the browser sends the
// photo straight to Cloud Storage. The admin's token only goes to the API.
import type { Api } from "./api.ts";
import { shrinkPhoto } from "./shrink.ts";

/** Something the admin can act on, worded for them. */
export class UploadError extends Error {
	constructor(message: string) {
		super(message);
		this.name = "UploadError";
	}
}

// The API's rules (api/media/media.go), checked first so a wrong file is
// caught before anything is sent.
const types = ["image/jpeg", "image/png", "image/webp"];
const maxBytes = 10 * 1024 * 1024;
const failed =
	"The photo didn't upload. Check your connection, then try again.";

interface Signed {
	key: string;
	uploadURL: string;
	headers: Record<string, string>;
}

/**
 * Uploads chosen (shrunk first, see shrink.ts) and returns its object name
 * for the post's coverImageKey.
 */
export async function uploadImage(
	api: Api,
	chosen: File,
	put: typeof fetch = fetch,
	shrink: (photo: File) => Promise<File> = shrinkPhoto,
): Promise<string> {
	if (!types.includes(chosen.type))
		throw new UploadError("Choose a JPEG, PNG or WebP photo");
	// The limit is on what's sent, so a big phone photo shrinks to fit.
	const file = await shrink(chosen);
	if (file.size > maxBytes)
		throw new UploadError("Choose a photo of 10 MB or smaller");

	const res = await api("/api/admin/uploads", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ contentType: file.type, size: file.size }),
	});
	if (res.status === 422) {
		const { fields = {} } = (await res.json()) as {
			fields?: Record<string, string>;
		};
		throw new UploadError(Object.values(fields)[0] ?? failed);
	}
	if (!res.ok) throw new UploadError(failed);
	const signed = (await res.json()) as Signed;

	try {
		const stored = await put(signed.uploadURL, {
			method: "PUT",
			headers: signed.headers,
			body: file,
		});
		if (!stored.ok) throw new UploadError(failed);
	} catch {
		throw new UploadError(failed);
	}
	return signed.key;
}
