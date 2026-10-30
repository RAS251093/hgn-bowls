// The guides for committee members (/admin?help). They're part of the
// page, so there's nothing to load.
import { el, type View } from "./dom";
import type { Guide } from "./posts";

export function showHelp(guide: Guide | null): View {
	el("[data-view=help]").hidden = false;
	document.title = `${guide === "events" ? "Adding an event" : guide === "posts" ? "Writing a news post" : "Help"} · Club admin`;
	return { leave: async () => {} };
}
