// The guides for committee members (/admin?help). They're part of the
// page, so there's nothing to load.
import { el, type View } from "./dom";
import type { Guide } from "./posts";

export function showHelp(guide: Guide | null): View {
	el("[data-view=help]").hidden = false;
	const titles: Record<Guide, string> = {
		posts: "Writing a news post",
		events: "Adding an event",
		admins: "Inviting an admin",
	};
	document.title = `${guide ? titles[guide] : "Help"} · Club admin`;
	return { leave: async () => {} };
}
