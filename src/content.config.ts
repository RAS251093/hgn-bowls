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
    const res = await fetch(import.meta.env.HYGRAPH_ENDPOINT, query);
    if (!res) {
      console.error("No posts returned from CMS");
      return [];
    }
    const json = await res.json();

    return json.data.posts.map(post => ({
      id: post.slug,
      ...post,
    }));
  },
});

export const collections = { posts };
