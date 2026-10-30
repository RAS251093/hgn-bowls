// The admin page as an app: sign in, then show the list or the editor
// for the address (?post=<id>), switching views in place. Links to other
// admin views are caught so the page never reloads, which would mean a
// trip through Auth0 (tokens live only in memory); the back button works
// through history.
import { apiUrl } from "../../config";
import { apiClient, SignInRequired } from "./api";
import { el, Forbidden, type View } from "./dom";
import { openEditor } from "./editor-view";
import { showList } from "./list-view";
import { editHref, type Route, routeOf } from "./posts";
import { startSession } from "./session";
import { showUndo } from "./toast";

function showState(state: "loading" | "not-admin" | null) {
	for (const s of document.querySelectorAll<HTMLElement>("[data-state]")) {
		s.hidden = s.dataset.state !== state;
	}
}

function hideViews() {
	for (const v of document.querySelectorAll<HTMLElement>("[data-view]")) {
		v.hidden = true;
	}
}

export async function start(): Promise<void> {
	try {
		const session = await startSession();
		if (!session) return; // on the way to Auth0

		const signOut = el<HTMLButtonElement>("[data-sign-out]");
		signOut.hidden = false;
		signOut.addEventListener("click", () => {
			signOut.disabled = true;
			void session.signOut();
		});
		const who = el("[data-signed-in-as]");
		who.textContent = session.name;
		who.hidden = false;

		const api = apiClient(apiUrl, session);
		let current: View | null = null;
		let moving = Promise.resolve();

		const render = async (route: Route, moved: boolean) => {
			await current?.leave();
			current = null;
			hideViews();
			showState(null);
			current =
				route.view === "list"
					? await showList(api)
					: await openEditor(api, route.id, {
							onCreated: (id) => history.replaceState(null, "", editHref(id)),
							onDeleted: (id) => void deleted(id),
						});
			if (route.view === "list") document.title = "News posts · Club admin";
			if (moved) {
				// Tell keyboard and screen reader users the view has changed.
				scrollTo(0, 0);
				const h1 = document.querySelector<HTMLElement>(
					"[data-view]:not([hidden]) h1",
				);
				h1?.setAttribute("tabindex", "-1");
				h1?.focus();
			}
		};

		// Back to the list, which offers Undo. The editor's history entry
		// becomes the list, so Back doesn't return to a deleted post.
		const deleted = async (id: string) => {
			history.replaceState(null, "", "/admin");
			await go({ view: "list" }, true);
			showUndo("Post deleted", async () => {
				await api(`/api/admin/posts/${encodeURIComponent(id)}/restore`, {
					method: "POST",
				});
				await go({ view: "list" }, false);
			});
		};

		// One move at a time, so leaving a view (and saving) finishes first.
		const go = (route: Route, moved: boolean) => {
			moving = moving.then(() => render(route, moved)).catch(fail);
			return moving;
		};

		document.addEventListener("click", (e) => {
			const link = (e.target as Element).closest<HTMLAnchorElement>("a[href]");
			if (
				!link ||
				e.defaultPrevented ||
				e.button !== 0 ||
				e.metaKey ||
				e.ctrlKey ||
				e.shiftKey ||
				e.altKey ||
				link.target
			)
				return;
			const url = new URL(link.href);
			if (
				url.origin !== location.origin ||
				url.pathname.replace(/\/$/, "") !== "/admin"
			)
				return;
			e.preventDefault();
			history.pushState(null, "", url.pathname + url.search);
			void go(routeOf(url.search), true);
		});
		addEventListener("popstate", () => void go(routeOf(location.search), true));

		await go(routeOf(location.search), false);
	} catch (err) {
		fail(err);
	}
}

function fail(err: unknown) {
	if (err instanceof SignInRequired) return;
	hideViews();
	if (err instanceof Forbidden) {
		showState("not-admin");
		return;
	}
	console.error(err);
	showState(null);
	el("[data-view=list]").hidden = false;
	el("[data-list-loading]").hidden = true;
	el("[data-list-error]").hidden = false;
}
