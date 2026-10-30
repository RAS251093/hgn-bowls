// Saves the post being edited after each pause in typing, so an admin never
// has to remember to. A new post is created on its first save (once it has
// a headline); after that each save sends If-Match with the version it last
// saw, so it can't overwrite someone else's changes. One save runs at a
// time; changes made during it are saved straight after.
import { type Api, SignInRequired } from "./api.ts";
import { type Draft, type FieldErrors, validateDraft } from "./draft.ts";

export type SaveStatus =
	| { kind: "saving" }
	| { kind: "saved"; at: Date }
	| { kind: "invalid"; fields: FieldErrors }
	/** Someone else saved the post since it was opened; autosave has stopped. */
	| { kind: "conflict" }
	/** The save didn't reach the API, or it failed; it's tried again on the next change. */
	| { kind: "failed" };

export interface AutosaveOptions {
	api: Api;
	/** The post being edited, or null for a new one. */
	post: { id: string; etag: string } | null;
	/** How long to wait after the last change, in ms. */
	delay?: number;
	onStatus: (status: SaveStatus) => void;
	/** Called once, when a new post is first saved. */
	onCreated?: (id: string) => void;
}

export class Autosave {
	private id: string | null;
	private etag: string | null;
	private pending: Draft | null = null;
	private saved = "";
	private stopped = false;
	private timer: ReturnType<typeof setTimeout> | undefined;
	private inflight: Promise<void> | null = null;
	private running: Promise<void> = Promise.resolve();

	private readonly options: AutosaveOptions;

	constructor(options: AutosaveOptions) {
		this.options = options;
		this.id = options.post?.id ?? null;
		this.etag = options.post?.etag ?? null;
	}

	/** Notes the latest draft and saves it after a pause. */
	update(draft: Draft): void {
		this.pending = draft;
		clearTimeout(this.timer);
		this.timer = setTimeout(() => {
			this.running = this.flush();
		}, this.options.delay ?? 1500);
	}

	/** Saves the latest draft now, after any save already under way. */
	async flush(): Promise<void> {
		clearTimeout(this.timer);
		while (this.inflight) await this.inflight;
		this.inflight = this.save().finally(() => {
			this.inflight = null;
		});
		await this.inflight;
	}

	/** True while the latest draft hasn't reached the API. */
	hasUnsavedChanges(): boolean {
		return this.pending !== null && JSON.stringify(this.pending) !== this.saved;
	}

	/** Resolves once a save started by the timer has finished. */
	idle(): Promise<void> {
		return this.running;
	}

	private async save(): Promise<void> {
		const draft = this.pending;
		if (!draft || this.stopped) return;
		const body = JSON.stringify(draft);
		if (body === this.saved) return;

		const fields = validateDraft(draft);
		if (Object.keys(fields).length > 0) {
			this.options.onStatus({ kind: "invalid", fields });
			return;
		}

		this.options.onStatus({ kind: "saving" });
		const headers = new Headers({ "Content-Type": "application/json" });
		if (this.id && this.etag) headers.set("If-Match", this.etag);
		let res: Response;
		try {
			res = await this.options.api(
				this.id
					? `/api/admin/posts/${encodeURIComponent(this.id)}`
					: "/api/admin/posts",
				{ method: this.id ? "PUT" : "POST", headers, body },
			);
		} catch (err) {
			if (err instanceof SignInRequired) return;
			this.options.onStatus({ kind: "failed" });
			return;
		}

		if (res.ok) {
			this.etag = res.headers.get("ETag");
			if (!this.id) {
				const { id } = (await res.json()) as { id: string };
				this.id = id;
				this.options.onCreated?.(id);
			}
			this.saved = body;
			this.options.onStatus({ kind: "saved", at: new Date() });
		} else if (res.status === 409) {
			this.stopped = true;
			this.options.onStatus({ kind: "conflict" });
		} else if (res.status === 422) {
			const { fields = {} } = (await res.json()) as { fields?: FieldErrors };
			this.options.onStatus({ kind: "invalid", fields });
		} else {
			this.options.onStatus({ kind: "failed" });
		}
	}
}
