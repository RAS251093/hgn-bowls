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

const posts = defineCollection({
  schema: z.object({
      title: z.string(),
      slug: z.string(),
      pubDate: z.string(),
      body: z.object({
        text: z.string()
      }),
      description: z.string(),
      altText: z.array(
       z.string()
      ),
      assets: z.array(
        z.string()
      )
  }),
  loader: async () => {
    const res = await client.request<{ posts: Post[] }>(`
      query Posts {
        posts {
          title
          slug
          pubDate
          body {
            text
          }
          description
          altText
          assets {
            id
          }
        }
      }`);

      if (!res.posts) {
        console.error("No posts returned from CMS");
        return [];
      }
      console.log(res)
      return res.posts.map(post => ({
        id: post.slug,
        ...post,
      }));
  },
});

export const collections = { posts };
