import { defineCollection, z } from "astro:content";

interface Post {
  title: string;
  slug: string;
  pubDate: string;
  body: { raw: JSON };
  assets: { id: string }[];
  description: string;
  altText: string[] | string;
}

const query = {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    query: `
    {
    posts {
          title
          description
          slug
          pubDate
          body {
            markdown
          }
          altText
          assets {
            id
            fileName
            url
          }
        }
    }`,
  }),
};

const posts = defineCollection({
  schema: z.object({
    title: z.string(),
    description: z.string(),
    slug: z.string(),
    pubDate: z.string(),
    body: z.object({
      markdown: z.string()
    }),
    altText: z.array(
      z.string()
    ),
    assets: z.array(
      z.object({
        id: z.string(),
        fileName: z.string(),
        url: z.string()
      })
    ).optional()
  }),
  loader: async () => {
    try {
      const endpoint = import.meta.env.HYGRAPH_ENDPOINT;
      if (!endpoint) {
        console.warn('HYGRAPH_ENDPOINT not set, returning empty posts array');
        return [];
      }

      const res = await fetch(endpoint, query);
      if (!res.ok) {
        console.error("Failed to fetch posts from CMS");
        return [];
      }

      const json = await res.json();
      return json.data.posts.map((post: Post) => ({
        id: post.slug,
        ...post,
      }));
    } catch (error) {
      console.error("Error loading posts:", error);
      return [];
    }
  },
});

export const collections = { posts };
