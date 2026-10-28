// What the news pages need from the posts collection.

import { getCollection } from "astro:content";
import type { ImageMetadata } from "astro";
import defaultCover from "../assets/two-crown-green-bowls.jpg";
import { newestFirst, type Post } from "./posts";

/** Published posts, newest first. */
export async function publishedPosts(): Promise<Post[]> {
	return newestFirst((await getCollection("posts")).map((entry) => entry.data));
}

/**
 * The image to show for a post: its own cover, or the club's photo of two
 * bowls on the green when it has none. The default is decorative (alt ""),
 * because it says nothing about the post.
 */
export function coverOf(post: Post): {
	src: ImageMetadata | string;
	alt: string;
	isDefault: boolean;
} {
	return post.cover
		? { src: post.cover.url, alt: post.cover.alt, isDefault: false }
		: { src: defaultCover, alt: "", isDefault: true };
}
