import { defineCollection, z } from "astro:content";
import { GraphQLClient } from "graphql-request";

interface Post {
	title: string;
	slug: string;
	pubDate: string;
	body?: { text?: string };
	assets?: { id?: string }[];
	description?: string;
	altText?: string[] | string;
}

const client = new GraphQLClient(import.meta.env.HYGRAPH_ENDPOINT);

const blog = defineCollection({
	// Load Markdown and MDX files in the `src/content/blog/` directory.
	loader: glob({ base: "./src/content/blog", pattern: "**/*.{md,mdx}" }),
	// Type-check frontmatter using a schema
	schema: z.object({
		title: z.string(),
		description: z.string(),
		// Transform string to Date object
		pubDate: z.coerce.date(),
		updatedDate: z.coerce.date().optional(),
		heroImage: z.string().optional(),
		altText: z.string().optional(),
	}),
});

export const collections = { blog };
