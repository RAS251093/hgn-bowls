// Publishing from the editor. Publish saves any pending changes, marks the
// post live and starts a website rebuild; the panel then follows the rebuild
// until the new build is live or has failed, offering Try again. Unpublish
// does the same in reverse. The post is saved whatever happens to the
// rebuild, and the panel says so.
import { deployPollMs } from "../../config";
import type { Api } from "./api";
import { type Deployment, deployState } from "./deploys";
import { el, Forbidden, template } from "./dom";

export interface PublishTarget {
	/** The post's id, or null while a new post hasn't been saved. */
	id: () => string | null;
	status: "draft" | "published";
	slug: string | null;
	/**
	 * Saves pending changes; resolves false (having shown why) if the post
	 * can't be published as it stands.
	 */
	ready: () => Promise<boolean>;
	/** Publishing moves the post on to a new version (its ETag). */
	setVersion: (etag: string) => void;
}

type Tone = "info" | "live" | "error";

const giveUpAfter = 15 * 60 * 1000;

export function publisher(view: HTMLElement, api: Api, post: PublishTarget) {
	const publish = el<HTMLButtonElement>("[data-publish]", view);
	const publishLabel = el("[data-publish-label]", publish);
	const unpublish = el<HTMLButtonElement>("[data-unpublish]", view);
	const badge = el("[data-editor-badge]", view);
	const panel = el("[data-publish-status]", view);
	const message = el("[data-publish-message]", panel);
	const link = el<HTMLAnchorElement>("[data-publish-link]", panel);
	const retry = el<HTMLButtonElement>("[data-publish-retry]", panel);

	let timer: ReturnType<typeof setTimeout> | undefined;
	let stopped = false;
	let lastSeen: string | null = null; // the newest build we know of
	let kind: "publish" | "unpublish" = "publish";

	const say = (
		tone: Tone,
		text: string,
		extra: "link" | "retry" | null = null,
	) => {
		panel.hidden = false;
		panel.dataset.tone = tone;
		message.textContent = text;
		link.hidden = extra !== "link";
		if (post.slug) link.href = `/news/${encodeURIComponent(post.slug)}`;
		retry.hidden = extra !== "retry";
	};

	const render = () => {
		const id = post.id();
		publish.hidden = !id;
		unpublish.hidden = !id || post.status !== "published";
		publishLabel.textContent =
			post.status === "published" ? "Update the website" : "Publish post";
		badge.replaceChildren(template(`[data-badge="${post.status}"]`));
	};

	const latest = async (): Promise<Deployment | null | undefined> => {
		try {
			const res = await api("/api/admin/deploys/latest");
			if (!res.ok) return undefined;
			const { deployment } = (await res.json()) as {
				deployment: Deployment | null;
			};
			return deployment;
		} catch {
			return undefined;
		}
	};

	const failedToStart = () =>
		say("error", "Saved, but the website didn't start updating.", "retry");

	// Follows the rebuild until a build newer than `before` is live or failed.
	const watch = (before: string | null) => {
		const started = Date.now();
		const poll = async () => {
			if (stopped) return;
			const d = await latest();
			if (d) lastSeen = d.id;
			const state = d === undefined ? "waiting" : deployState(before, d);
			if (state === "live") {
				if (kind === "publish") say("live", "Live on the website.", "link");
				else say("live", "The post is off the website.");
				return;
			}
			if (state === "failed") {
				say(
					"error",
					`The website update failed. Your post is saved; try again, and if it fails again, tell the website's developer.`,
					"retry",
				);
				return;
			}
			if (Date.now() - started > giveUpAfter) {
				say(
					"info",
					"This is taking longer than usual. Check the website in a few minutes.",
				);
				return;
			}
			timer = setTimeout(poll, deployPollMs);
		};
		timer = setTimeout(poll, deployPollMs);
	};

	const updating = () =>
		say(
			"info",
			kind === "publish"
				? "Updating the website… This usually takes a minute or two."
				: "Taking the post off the website…",
		);

	const change = async (action: "publish" | "unpublish") => {
		const id = post.id();
		if (!id) return;
		clearTimeout(timer);
		publish.disabled = unpublish.disabled = true;
		try {
			if (action === "publish" && !(await post.ready())) return;
			kind = action;
			const before = await latest();
			lastSeen = before?.id ?? null;
			updating();
			let res: Response;
			try {
				res = await api(
					`/api/admin/posts/${encodeURIComponent(id)}/${action}`,
					{
						method: "POST",
					},
				);
			} catch {
				say(
					"error",
					`We couldn't ${action} the post. Check your connection, then try again.`,
				);
				return;
			}
			if (res.status === 403) throw new Forbidden();
			if (!res.ok) {
				say("error", `We couldn't ${action} the post. Try again in a moment.`);
				return;
			}
			const etag = res.headers.get("ETag");
			if (etag) post.setVersion(etag);
			const { post: saved, rebuild } = (await res.json()) as {
				post: { status: "draft" | "published"; slug: string | null };
				rebuild: { status: "started" | "failed" | "skipped" };
			};
			post.status = saved.status;
			post.slug = saved.slug;
			render();
			if (rebuild.status === "failed") failedToStart();
			else if (rebuild.status === "skipped") panel.hidden = true;
			else watch(lastSeen);
		} finally {
			publish.disabled = unpublish.disabled = false;
		}
	};

	const onRetry = async () => {
		clearTimeout(timer);
		retry.disabled = true;
		const before = lastSeen;
		updating();
		try {
			const res = await api("/api/admin/rebuild", { method: "POST" });
			if (!res.ok) failedToStart();
			else watch(before);
		} catch {
			failedToStart();
		} finally {
			retry.disabled = false;
		}
	};

	const onPublish = () => void change("publish");
	const onUnpublish = () => void change("unpublish");
	publish.addEventListener("click", onPublish);
	unpublish.addEventListener("click", onUnpublish);
	retry.addEventListener("click", onRetry);
	panel.hidden = true;
	render();

	return {
		/** Call when a new post is first saved, so it can be published. */
		render,
		/** Problems found before publishing. */
		blocked: (text: string) => say("error", text),
		stop: () => {
			stopped = true;
			clearTimeout(timer);
			publish.removeEventListener("click", onPublish);
			unpublish.removeEventListener("click", onUnpublish);
			retry.removeEventListener("click", onRetry);
		},
	};
}
