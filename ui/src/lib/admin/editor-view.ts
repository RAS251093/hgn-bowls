// The post editor: headline, description and the post itself as rich text.
// Admins never see Markdown: the toolbar formats text, and the body is
// turned into Markdown only when it's saved. Saving happens by itself after
// each pause in typing (autosave.ts); the status line says how it went.
import { Editor } from "@tiptap/core";
import { renderPost } from "../markdown";
import type { Api } from "./api";
import { Autosave, type SaveStatus } from "./autosave";
import { el, Forbidden, type View } from "./dom";
import { type Draft, type FieldErrors, validateDraft } from "./draft";
import { linkAddress } from "./links";
import { editorExtensions, fromMarkdown, toMarkdown } from "./markdown";
import { UploadError, uploadImage } from "./upload";

interface StoredPost extends Draft {
	id: string;
	/** A link to the cover valid for an hour. */
	coverUrl?: string;
}

export interface EditorOptions {
	/** Called once a new post has been saved for the first time. */
	onCreated: (id: string) => void;
	/** Called once the post has been deleted, to leave the editor. */
	onDeleted: (id: string) => void;
}

const london = new Intl.DateTimeFormat("en-GB", {
	timeZone: "Europe/London",
	hour: "2-digit",
	minute: "2-digit",
	hourCycle: "h23",
});

// Shows or clears a field's error beside it (Field with liveErrors).
function fieldError(control: HTMLElement, message: string | undefined) {
	const box = control
		.closest(".group")
		?.querySelector<HTMLElement>("[data-field-error]");
	if (message) control.setAttribute("aria-invalid", "true");
	else control.removeAttribute("aria-invalid");
	if (!box) return;
	box.hidden = !message;
	el("[data-message]", box).textContent = message ?? "";
}

export async function openEditor(
	api: Api,
	id: string | null,
	{ onCreated, onDeleted }: EditorOptions,
): Promise<View> {
	const view = el("[data-view=edit]");
	const form = el<HTMLFormElement>("[data-editor-form]", view);
	const heading = el("[data-editor-heading]", view);
	const statusLine = el("[data-save-status]", view);
	const conflict = el("[data-conflict]", view);
	const loadError = el("[data-editor-error]", view);
	const loading = el("[data-editor-loading]", view);
	const title = el<HTMLInputElement>("#post-title", view);
	const description = el<HTMLTextAreaElement>("#post-description", view);
	const bodyError = el("[data-body-error]", view);
	const mount = el("[data-editor-mount]", view);

	view.hidden = false;
	form.hidden = true;
	conflict.hidden = true;
	loadError.hidden = true;
	statusLine.textContent = "";
	heading.textContent = id ? "Edit post" : "Write a post";
	document.title = `${heading.textContent} · Club admin`;
	const nothing: View = { leave: async () => {} };

	let post: StoredPost | null = null;
	let etag = "";
	if (id) {
		loading.hidden = false;
		const res = await api(`/api/admin/posts/${encodeURIComponent(id)}`).finally(
			() => {
				loading.hidden = true;
			},
		);
		if (res.status === 403) throw new Forbidden();
		if (!res.ok) {
			loadError.hidden = false;
			return nothing;
		}
		post = (await res.json()) as StoredPost;
		etag = res.headers.get("ETag") ?? "";
	}

	title.value = post?.title ?? "";
	description.value = post?.description ?? "";
	for (const control of [title, description]) fieldError(control, undefined);
	bodyError.hidden = true;
	form.hidden = false;

	const editor = new Editor({
		element: mount,
		extensions: editorExtensions(),
		content: fromMarkdown(post?.bodyMd ?? ""),
		editorProps: {
			attributes: {
				id: "post-body",
				role: "textbox",
				"aria-multiline": "true",
				"aria-labelledby": "post-body-label",
				"aria-describedby": "post-body-error",
				class: `${mount.dataset.prose ?? ""} min-h-80 p-4`,
			},
		},
	});

	// The headline only counts as missing once the admin has been there, or
	// for a post that already had one.
	let titleTouched = Boolean(post);
	const cover = coverPicker(view, api, post, () => changed());
	const draft = (): Draft => ({
		title: title.value,
		description: description.value,
		bodyMd: toMarkdown(editor.getJSON()),
		coverImageKey: cover.key(),
		coverAlt: cover.key() ? cover.alt() : null,
	});

	const showErrors = (errors: FieldErrors) => {
		fieldError(title, titleTouched ? errors.title : undefined);
		fieldError(description, errors.description);
		cover.showError(errors.coverAlt);
		bodyError.textContent = errors.bodyMd ?? "";
		bodyError.hidden = !errors.bodyMd;
		el("#post-body", view).toggleAttribute(
			"aria-invalid",
			Boolean(errors.bodyMd),
		);
	};

	let retry: ReturnType<typeof setTimeout> | undefined;
	let isNew = !post;
	let postId = post?.id ?? null;
	const deleteRow = el("[data-delete-row]", view);
	const deleteError = el("[data-delete-error]", view);
	const deleteButton = el<HTMLButtonElement>("[data-delete]", view);
	const previewToggle = el<HTMLButtonElement>("[data-preview-toggle]", view);
	deleteRow.hidden = !postId;
	deleteError.hidden = true;
	deleteButton.disabled = false;
	previewToggle.hidden = false;
	const saver = new Autosave({
		api,
		post: post ? { id: post.id, etag } : null,
		onCreated: (newId) => {
			isNew = false;
			postId = newId;
			deleteRow.hidden = false;
			previewToggle.hidden = false;
			heading.textContent = "Edit post";
			document.title = "Edit post · Club admin";
			onCreated(newId);
		},
		onStatus: (status: SaveStatus) => {
			clearTimeout(retry);
			switch (status.kind) {
				case "saving":
					statusLine.textContent = "Saving…";
					break;
				case "saved":
					statusLine.textContent = `Saved ${london.format(status.at)}`;
					break;
				case "invalid":
					showErrors(status.fields);
					statusLine.textContent = invalidMessage(status.fields);
					break;
				case "failed":
					statusLine.textContent = "Not saved. We'll try again in a moment.";
					retry = setTimeout(() => void saver.flush(), 10_000);
					break;
				case "conflict":
					statusLine.textContent = "Not saved";
					conflict.hidden = false;
					break;
			}
		},
	});

	// What the status line says when a save is held back.
	const invalidMessage = (fields: FieldErrors) => {
		const only = Object.keys(fields).length === 1;
		if (isNew && fields.title === "Add a headline")
			return "Add a headline to save";
		if (only && fields.coverAlt && !cover.altTouched())
			return "Describe the cover photo to save";
		return "Not saved. Check the highlighted fields.";
	};

	const changed = () => {
		const d = draft();
		showErrors(validateDraft(d));
		saver.update(d);
	};
	const onTitle = () => {
		titleTouched = true;
		changed();
	};
	title.addEventListener("input", onTitle);
	description.addEventListener("input", changed);
	editor.on("update", changed);

	const stopToolbar = toolbar(view, editor);
	const stopPreview = previewer(view, form, () => ({
		draft: draft(),
		coverSrc: cover.src(),
	}));

	// Deleting acts at once; the list then offers Undo. Changes are saved
	// first, so Undo brings them back too.
	const onDelete = async () => {
		if (!postId) return;
		deleteButton.disabled = true;
		deleteError.hidden = true;
		clearTimeout(retry);
		await saver.flush();
		let ok = false;
		try {
			const res = await api(`/api/admin/posts/${encodeURIComponent(postId)}`, {
				method: "DELETE",
			});
			if (res.status === 403) throw new Forbidden();
			ok = res.ok;
		} catch (err) {
			if (err instanceof Forbidden) throw err;
		}
		if (!ok) {
			deleteButton.disabled = false;
			deleteError.hidden = false;
			return;
		}
		onDeleted(postId);
	};
	deleteButton.addEventListener("click", onDelete);

	const warn = (e: BeforeUnloadEvent) => {
		if (saver.hasUnsavedChanges()) e.preventDefault();
	};
	addEventListener("beforeunload", warn);

	return {
		async leave() {
			clearTimeout(retry);
			await saver.flush();
			title.removeEventListener("input", onTitle);
			description.removeEventListener("input", changed);
			removeEventListener("beforeunload", warn);
			stopToolbar();
			stopPreview();
			deleteButton.removeEventListener("click", onDelete);
			cover.stop();
			editor.destroy();
		},
	};
}

// The cover photo: shows the current one, uploads a new one (straight to
// Cloud Storage) or removes it, and holds its description. A new photo
// needs a new description, so choosing one clears the old.
function coverPicker(
	view: HTMLElement,
	api: Api,
	post: StoredPost | null,
	onChange: () => void,
) {
	const box = el("[data-cover]", view);
	const set = el("[data-cover-set]", box);
	const empty = el("[data-cover-empty]", box);
	const image = el<HTMLImageElement>("[data-cover-image]", box);
	const alt = el<HTMLInputElement>("#cover-alt", box);
	const uploading = el("[data-cover-uploading]", box);
	const problem = el("[data-cover-error]", box);
	const files = [
		...box.querySelectorAll<HTMLInputElement>("[data-cover-file]"),
	];

	let key = post?.coverImageKey ?? null;
	let preview = ""; // an object URL for a photo chosen here
	let touched = Boolean(post?.coverAlt);
	alt.value = post?.coverAlt ?? "";

	const unavailable = el("[data-cover-unavailable]", box);

	// No link (the API couldn't sign one) or a link that won't load: say
	// so, rather than show a broken image.
	const cantShow = () => {
		image.hidden = true;
		image.removeAttribute("src");
		unavailable.hidden = false;
	};
	const render = () => {
		set.hidden = !key;
		empty.hidden = Boolean(key);
		const src = key ? preview || post?.coverUrl : "";
		if (!src) {
			if (key) cantShow();
			else image.removeAttribute("src");
			return;
		}
		image.hidden = false;
		unavailable.hidden = true;
		image.src = src;
	};
	const say = (message: string) => {
		problem.textContent = message;
		problem.hidden = !message;
	};

	const onFile = async (e: Event) => {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		input.value = "";
		if (!file) return;
		say("");
		uploading.hidden = false;
		for (const f of files) f.disabled = true;
		try {
			key = await uploadImage(api, file);
			if (preview) URL.revokeObjectURL(preview);
			preview = URL.createObjectURL(file);
			alt.value = "";
			touched = false;
			render();
			onChange();
			alt.focus();
		} catch (err) {
			say(
				err instanceof UploadError
					? err.message
					: "The photo didn't upload. Try again.",
			);
		} finally {
			uploading.hidden = true;
			for (const f of files) f.disabled = false;
		}
	};
	const onRemove = () => {
		key = null;
		alt.value = "";
		touched = false;
		say("");
		render();
		onChange();
		files.find((f) => !f.closest("[hidden]"))?.focus();
	};
	const onAlt = () => {
		touched = true;
		onChange();
	};

	image.addEventListener("error", cantShow);
	for (const f of files) f.addEventListener("change", onFile);
	const remove = el("[data-cover-remove]", box);
	remove.addEventListener("click", onRemove);
	alt.addEventListener("input", onAlt);
	fieldError(alt, undefined);
	say("");
	uploading.hidden = true;
	render();

	return {
		key: () => key,
		alt: () => alt.value,
		/** What to show for the cover: the photo, or "" if it can't be shown. */
		src: () => (key ? preview || post?.coverUrl || "" : ""),
		altTouched: () => touched,
		showError: (message: string | undefined) =>
			fieldError(alt, touched ? message : undefined),
		stop: () => {
			image.removeEventListener("error", cantShow);
			for (const f of files) f.removeEventListener("change", onFile);
			remove.removeEventListener("click", onRemove);
			alt.removeEventListener("input", onAlt);
			if (preview) URL.revokeObjectURL(preview);
		},
	};
}

// The Preview button: swaps the form for the post as members will see it,
// unsaved changes included, rendered by the site's own Markdown pipeline
// (so it's sanitised the same way). A post without a cover shows the club
// photo, as on the site.
function previewer(
	view: HTMLElement,
	form: HTMLElement,
	current: () => { draft: Draft; coverSrc: string },
): () => void {
	const toggle = el<HTMLButtonElement>("[data-preview-toggle]", view);
	const label = el("[data-preview-label]", toggle);
	const panel = el("[data-preview]", view);
	const cover = el<HTMLImageElement>("[data-preview-cover]", panel);
	let showing = false;

	const show = async (on: boolean) => {
		showing = on;
		if (on) {
			const { draft, coverSrc } = current();
			el("[data-preview-title]", panel).textContent = draft.title.trim();
			const description = el("[data-preview-description]", panel);
			description.textContent = draft.description.trim();
			description.hidden = !draft.description.trim();
			const custom = Boolean(draft.coverImageKey && coverSrc);
			cover.src = custom ? coverSrc : (cover.dataset.default ?? "");
			cover.alt = custom ? (draft.coverAlt ?? "") : "";
			// renderPost's output is sanitised, so setting it as HTML is safe.
			el("[data-preview-body]", panel).innerHTML = await renderPost(
				draft.bodyMd,
			);
		}
		panel.hidden = !on;
		form.hidden = on;
		label.textContent = on ? "Back to editing" : "Preview";
		for (const icon of toggle.querySelectorAll<SVGElement>("[data-when]")) {
			icon.toggleAttribute(
				"hidden",
				icon.dataset.when !== (on ? "previewing" : "editing"),
			);
		}
	};
	const onToggle = () => void show(!showing);
	toggle.addEventListener("click", onToggle);
	void show(false);
	return () => {
		toggle.removeEventListener("click", onToggle);
		panel.hidden = true;
	};
}

// The formatting buttons, their pressed states, and the Link form.
function toolbar(view: HTMLElement, editor: Editor): () => void {
	const buttons = [...view.querySelectorAll<HTMLButtonElement>("[data-tool]")];
	const linkForm = el("[data-link-form]", view);
	const linkInput = el<HTMLInputElement>("#post-link", view);

	const commands: Record<string, () => void> = {
		h2: () => editor.chain().focus().toggleHeading({ level: 2 }).run(),
		h3: () => editor.chain().focus().toggleHeading({ level: 3 }).run(),
		bold: () => editor.chain().focus().toggleBold().run(),
		italic: () => editor.chain().focus().toggleItalic().run(),
		bulletList: () => editor.chain().focus().toggleBulletList().run(),
		orderedList: () => editor.chain().focus().toggleOrderedList().run(),
		quote: () => editor.chain().focus().toggleBlockquote().run(),
		link: () => openLink(),
	};
	const isActive: Record<string, () => boolean> = {
		h2: () => editor.isActive("heading", { level: 2 }),
		h3: () => editor.isActive("heading", { level: 3 }),
		bold: () => editor.isActive("bold"),
		italic: () => editor.isActive("italic"),
		link: () => editor.isActive("link"),
		bulletList: () => editor.isActive("bulletList"),
		orderedList: () => editor.isActive("orderedList"),
		quote: () => editor.isActive("blockquote"),
	};

	const refresh = () => {
		for (const b of buttons) {
			const active = isActive[b.dataset.tool ?? ""]?.() ?? false;
			b.setAttribute("aria-pressed", String(active));
		}
	};

	const openLink = () => {
		linkForm.hidden = false;
		linkInput.value = editor.getAttributes("link").href ?? "";
		fieldError(linkInput, undefined);
		el("[data-link-remove]", linkForm).hidden = !editor.isActive("link");
		linkInput.focus();
	};
	const closeLink = () => {
		linkForm.hidden = true;
		editor.commands.focus();
	};
	const addLink = () => {
		const href = linkAddress(linkInput.value);
		if (!href) {
			fieldError(
				linkInput,
				"Enter a web address, like example.org, or an email address",
			);
			return;
		}
		const chain = editor.chain().focus();
		if (editor.state.selection.empty && !editor.isActive("link")) {
			chain
				.insertContent({
					type: "text",
					text: linkInput.value.trim(),
					marks: [{ type: "link", attrs: { href } }],
				})
				.run();
		} else {
			chain.extendMarkRange("link").setLink({ href }).run();
		}
		linkForm.hidden = true;
	};
	const removeLink = () => {
		editor.chain().focus().extendMarkRange("link").unsetLink().run();
		linkForm.hidden = true;
	};

	const onClick = (e: Event) => {
		const button = (e.target as Element).closest<HTMLButtonElement>(
			"[data-tool]",
		);
		if (button) commands[button.dataset.tool ?? ""]?.();
		if ((e.target as Element).closest("[data-link-add]")) addLink();
		if ((e.target as Element).closest("[data-link-remove]")) removeLink();
		if ((e.target as Element).closest("[data-link-cancel]")) closeLink();
	};
	const onKey = (e: KeyboardEvent) => {
		if (e.target !== linkInput) return;
		if (e.key === "Enter") {
			e.preventDefault();
			addLink();
		} else if (e.key === "Escape") {
			closeLink();
		}
	};

	// A pointer press on a formatting button mustn't take focus from the
	// post, or the next keystrokes go to the button. (Keyboard users still
	// reach the buttons with Tab; the command then hands focus back.)
	const keepFocus = (e: MouseEvent) => {
		const tool = (e.target as Element).closest<HTMLElement>("[data-tool]");
		if (tool && tool.dataset.tool !== "link") e.preventDefault();
	};

	view.addEventListener("mousedown", keepFocus);
	view.addEventListener("click", onClick);
	view.addEventListener("keydown", onKey);
	editor.on("transaction", refresh);
	linkForm.hidden = true;
	refresh();
	return () => {
		view.removeEventListener("mousedown", keepFocus);
		view.removeEventListener("click", onClick);
		view.removeEventListener("keydown", onKey);
	};
}
