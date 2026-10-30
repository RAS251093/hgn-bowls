// The list of news posts, most recently changed first. Each row is a link
// to the editor; the app catches the click, so the page never reloads.
import type { Api } from "./api";
import { el, Forbidden, template, type View } from "./dom";
import { type AdminPost, editHref, metaLine } from "./posts";

function row(post: AdminPost): DocumentFragment {
	const li = template("[data-row]");
	const link = el<HTMLAnchorElement>("[data-title]", li);
	link.textContent = post.title;
	link.href = editHref(post.id);
	el("[data-status]", li).replaceChildren(
		template(
			`[data-badge="${post.status === "published" ? "published" : "draft"}"]`,
		),
	);
	el("[data-meta]", li).textContent = metaLine(post);
	return li;
}

export async function showList(api: Api): Promise<View> {
	const view = el("[data-view=list]");
	const loading = el("[data-list-loading]", view);
	const failed = el("[data-list-error]", view);
	const ready = view.querySelectorAll<HTMLElement>("[data-ready]");
	view.hidden = false;
	loading.hidden = false;
	failed.hidden = true;
	for (const r of ready) r.hidden = true;
	const leave = async () => {};

	const res = await api("/api/admin/posts");
	loading.hidden = true;
	if (res.status === 403) throw new Forbidden();
	if (!res.ok) {
		failed.hidden = false;
		return { leave };
	}
	const { posts } = (await res.json()) as { posts: AdminPost[] };
	const list = el<HTMLUListElement>("[data-list]", view);
	list.replaceChildren(...posts.map(row));
	list.hidden = posts.length === 0;
	el("[data-empty]", view).hidden = posts.length > 0;
	for (const r of ready) r.hidden = false;
	return { leave };
}
