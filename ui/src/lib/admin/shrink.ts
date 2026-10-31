// Shrinks a cover photo in the browser before it's uploaded: phone photos
// are often 4000px and several MB, and the admin list and editor load the
// original. The site shows covers at up to 1000px (2000px on sharp
// screens), so 2000px WebP loses nothing anyone will see.

const longestSide = 2000;
const quality = 0.82;

/** The size that fits within max on its longest side, keeping the shape. */
export function fitWithin(
	width: number,
	height: number,
	max: number,
): { width: number; height: number } {
	const scale = Math.min(1, max / Math.max(width, height));
	return {
		width: Math.max(1, Math.round(width * scale)),
		height: Math.max(1, Math.round(height * scale)),
	};
}

/**
 * A smaller WebP copy of photo, or photo itself if the browser can't make
 * one or the copy wouldn't be smaller.
 */
export async function shrinkPhoto(photo: File): Promise<File> {
	try {
		if (typeof createImageBitmap !== "function") return photo;
		// Turned the right way up, as phones save it (EXIF orientation).
		const bitmap = await createImageBitmap(photo, {
			imageOrientation: "from-image",
		});
		const { width, height } = fitWithin(
			bitmap.width,
			bitmap.height,
			longestSide,
		);
		const canvas = document.createElement("canvas");
		canvas.width = width;
		canvas.height = height;
		canvas.getContext("2d")?.drawImage(bitmap, 0, 0, width, height);
		bitmap.close();
		const blob = await new Promise<Blob | null>((resolve) =>
			canvas.toBlob(resolve, "image/webp", quality),
		);
		// Some browsers quietly fall back to PNG; only a smaller WebP will do.
		if (blob?.type !== "image/webp" || blob.size >= photo.size)
			return photo;
		const name = `${photo.name.replace(/\.[^.]+$/, "") || "photo"}.webp`;
		return new File([blob], name, { type: "image/webp" });
	} catch {
		return photo;
	}
}
