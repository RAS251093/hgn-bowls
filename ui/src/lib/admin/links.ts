// The address an admin types into the Link form, tidied into a safe link:
// web addresses (https:// added if left off) and email addresses only.

export function linkAddress(typed: string): string | null {
	const s = typed.trim();
	if (!s || /\s/.test(s)) return null;
	if (/^mailto:/i.test(s) || /^[^@/:]+@[^@/]+\.[^@/]+$/.test(s)) {
		const address = s.replace(/^mailto:/i, "");
		return /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(address)
			? `mailto:${address}`
			: null;
	}
	let url: URL;
	try {
		url = new URL(/^[a-z][a-z\d+.-]*:/i.test(s) ? s : `https://${s}`);
	} catch {
		return null;
	}
	if (url.protocol !== "https:" && url.protocol !== "http:") return null;
	if (!url.hostname.includes(".")) return null;
	return url.href;
}
