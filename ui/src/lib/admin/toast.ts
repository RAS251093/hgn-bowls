// The undo toast (UndoToast): one at a time, bottom-left, gone after six
// seconds unless Undo is pressed. The countdown pauses while the pointer
// or keyboard focus is on it, so there's time to reach Undo.
import { el, template } from "./dom";

const lasts = 6000;
let dismissCurrent: (() => void) | null = null;

export function showUndo(message: string, onUndo: () => void): void {
	dismissCurrent?.();
	const slot = el("[data-toast-slot]");
	slot.replaceChildren(template("[data-undo-toast]"));
	const toast = slot.firstElementChild as HTMLElement;
	el("p", toast).textContent = message;
	const bar = toast.querySelector<HTMLElement>("[aria-hidden=true]");

	let remaining = lasts;
	let started = Date.now();
	let timer: ReturnType<typeof setTimeout> | undefined;
	const dismiss = () => {
		clearTimeout(timer);
		toast.remove();
		if (dismissCurrent === dismiss) dismissCurrent = null;
	};
	const run = () => {
		started = Date.now();
		timer = setTimeout(dismiss, remaining);
		if (bar) bar.style.animationPlayState = "running";
	};
	const pause = () => {
		clearTimeout(timer);
		remaining -= Date.now() - started;
		if (bar) bar.style.animationPlayState = "paused";
	};
	let paused = false;
	const hold = () => {
		if (!paused) pause();
		paused = true;
	};
	const release = () => {
		if (toast.matches(":hover") || toast.contains(document.activeElement))
			return;
		if (paused) run();
		paused = false;
	};

	toast.addEventListener("mouseenter", hold);
	toast.addEventListener("focusin", hold);
	toast.addEventListener("mouseleave", release);
	toast.addEventListener("focusout", () => setTimeout(release));
	el("[data-undo]", toast).addEventListener("click", () => {
		dismiss();
		onUndo();
	});
	dismissCurrent = dismiss;
	run();
}
