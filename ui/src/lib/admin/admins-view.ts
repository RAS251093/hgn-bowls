// The admins (/admin?admins): who can sign in, inviting someone new,
// sending an invitation again and removing access. Only people whose token
// grants manage:admins see the way here, and the API checks it again.
// Removing access acts at once and offers Undo, which gives it back.
import { type Admin, isYou, type Me, statusLine, withAdmin } from "./admins";
import { type Api, SignInRequired } from "./api";
import { el, template, type View } from "./dom";
import { showUndo } from "./toast";

const offline = "Check your connection, then try again.";

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

// The API's own words for what went wrong, when it sends them.
async function errorOf(res: Response): Promise<string | undefined> {
	try {
		const body = (await res.json()) as { error?: unknown };
		return typeof body.error === "string" ? body.error : undefined;
	} catch {
		return undefined;
	}
}

const path = (id: string) => `/api/admin/admins/${encodeURIComponent(id)}`;

export async function showAdmins(api: Api, me: Me): Promise<View> {
	const view = el("[data-view=admins]");
	const loading = el("[data-admins-loading]", view);
	const ready = el("[data-admins-ready]", view);
	const denied = el("[data-admins-denied]", view);
	const unavailable = el("[data-admins-unavailable]", view);
	const failed = el("[data-admins-error]", view);
	const form = el<HTMLFormElement>("[data-invite-form]", view);
	const name = el<HTMLInputElement>("#invite-name", form);
	const email = el<HTMLInputElement>("#invite-email", form);
	const send = el<HTMLButtonElement>("[data-invite-send]", form);
	const status = el("[data-invite-status]", view);
	const alert = el("[data-admins-alert]", view);
	const list = el<HTMLUListElement>("[data-admins]", view);

	document.title = "Admins · Club admin";
	view.hidden = false;
	loading.hidden = false;
	for (const box of [ready, denied, unavailable, failed, status, alert]) {
		box.hidden = true;
	}
	form.reset();
	for (const control of [name, email]) fieldError(control, undefined);
	list.replaceChildren();

	const stop = new AbortController();
	let left = false;
	const leave = async () => {
		left = true;
		stop.abort();
	};

	const res = await api("/api/admin/admins");
	loading.hidden = true;
	if (res.status === 403) {
		denied.hidden = false;
		return { leave };
	}
	if (res.status === 503) {
		unavailable.hidden = false;
		return { leave };
	}
	if (!res.ok) {
		failed.hidden = false;
		return { leave };
	}
	let admins = ((await res.json()) as { admins: Admin[] }).admins;
	// What a row says after an action on it, kept as the list redraws.
	const notes = new Map<string, string>();

	const showAlert = (message: string | null) => {
		alert.hidden = !message;
		el("[data-admins-alert-message]", alert).textContent = message ?? "";
	};

	const showStatus = (tone: "sent" | "failed" | null, message = "") => {
		status.hidden = !tone;
		if (tone) status.dataset.tone = tone;
		status.textContent = message;
	};

	const row = (admin: Admin): DocumentFragment => {
		const li = template("[data-admin-row]");
		el("[data-admin-name]", li).textContent = admin.name;
		el("[data-admin-email]", li).textContent = admin.email;
		el("[data-admin-status]", li).textContent = statusLine(admin);
		el("[data-admin-note]", li).textContent = notes.get(admin.id) ?? "";
		const resend = el<HTMLButtonElement>("[data-resend]", li);
		const remove = el<HTMLButtonElement>("[data-remove]", li);
		const you = isYou(admin, me);
		resend.hidden = admin.status !== "invited";
		remove.hidden = you;
		el("[data-you]", li).hidden = !you;
		const label = `${admin.name}, ${admin.email}`;
		resend.setAttribute("aria-description", label);
		remove.setAttribute("aria-description", label);
		resend.addEventListener("click", () => void resendTo(admin, resend));
		remove.addEventListener("click", () => void removeAccess(admin, remove));
		return li;
	};

	const draw = () => {
		if (left) return;
		list.replaceChildren(...admins.map(row));
	};

	// Updates the row in place, so the focused button stays where it is.
	const note = (admin: Admin, button: HTMLElement, message: string) => {
		notes.set(admin.id, message);
		const line = button.closest("li")?.querySelector("[data-admin-note]");
		if (line) line.textContent = message;
	};

	// Marked busy rather than disabled, which would take focus off it.
	const resendTo = async (admin: Admin, button: HTMLButtonElement) => {
		if (button.ariaBusy === "true") return;
		button.ariaBusy = "true";
		showAlert(null);
		note(admin, button, "");
		try {
			const res = await api(`${path(admin.id)}/invite`, { method: "POST" });
			if (res.ok) {
				note(admin, button, "Email sent again");
			} else {
				showAlert(
					(await errorOf(res)) ??
						`We couldn't send the email to ${admin.email}. Try again in a minute.`,
				);
			}
		} catch (err) {
			if (err instanceof SignInRequired) return;
			showAlert(`We couldn't send the email. ${offline}`);
		}
		button.ariaBusy = null;
	};

	const removeAccess = async (admin: Admin, button: HTMLButtonElement) => {
		button.disabled = true;
		showAlert(null);
		try {
			const res = await api(path(admin.id), { method: "DELETE" });
			if (!res.ok) {
				showAlert(
					(await errorOf(res)) ?? `We couldn't remove their access. ${offline}`,
				);
				button.disabled = false;
				return;
			}
		} catch (err) {
			if (err instanceof SignInRequired) return;
			showAlert(`We couldn't remove their access. ${offline}`);
			button.disabled = false;
			return;
		}
		admins = admins.filter((a) => a.id !== admin.id);
		notes.delete(admin.id);
		draw();
		// The button has gone, so keep focus on the list rather than lose it.
		list.focus();
		showUndo("Access removed", () => void giveBack(admin));
	};

	const giveBack = async (admin: Admin) => {
		showAlert(null);
		let message = `We couldn't give ${admin.name} their access back. ${offline}`;
		try {
			const res = await api(path(admin.id), { method: "PUT" });
			if (res.ok) {
				const body = (await res.json()) as { admin?: Admin };
				admins = withAdmin(admins, body.admin ?? admin);
				draw();
				return;
			}
			message = (await errorOf(res)) ?? message;
		} catch (err) {
			if (err instanceof SignInRequired) return;
		}
		if (!left) showAlert(message);
	};

	const invite = async (e: SubmitEvent) => {
		e.preventDefault();
		if (send.disabled) return;
		showAlert(null);
		showStatus(null);
		for (const control of [name, email]) fieldError(control, undefined);
		send.disabled = true;
		try {
			const res = await api("/api/admin/admins", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					name: name.value.trim(),
					email: email.value.trim(),
				}),
			});
			if (res.status === 201) {
				const body = (await res.json()) as {
					admin: Admin;
					email: { status: "sent" | "failed" };
				};
				admins = withAdmin(admins, body.admin);
				notes.delete(body.admin.id);
				draw();
				form.reset();
				if (body.email.status === "sent") {
					showStatus(
						"sent",
						`We've emailed ${body.admin.email} a link to choose a password. Ask them to check their spam folder if it hasn't arrived.`,
					);
				} else {
					showStatus(
						"failed",
						`${body.admin.name} can now be an admin, but the email didn't send. Press Send the email again by their name to try again.`,
					);
				}
			} else if (res.status === 422) {
				const body = (await res.json()) as {
					error?: string;
					fields?: { name?: string; email?: string };
				};
				fieldError(name, body.fields?.name);
				fieldError(email, body.fields?.email);
				const first = body.fields?.name
					? name
					: body.fields?.email
						? email
						: null;
				if (first) first.focus();
				else
					showAlert(
						body.error ?? `We couldn't send the invitation. ${offline}`,
					);
			} else {
				showAlert(
					(await errorOf(res)) ?? `We couldn't send the invitation. ${offline}`,
				);
			}
		} catch (err) {
			if (err instanceof SignInRequired) return;
			showAlert(`We couldn't send the invitation. ${offline}`);
		} finally {
			send.disabled = false;
		}
	};

	form.addEventListener("submit", (e) => void invite(e), {
		signal: stop.signal,
	});
	for (const control of [name, email]) {
		control.addEventListener("input", () => fieldError(control, undefined), {
			signal: stop.signal,
		});
	}

	draw();
	ready.hidden = false;
	return { leave };
}
