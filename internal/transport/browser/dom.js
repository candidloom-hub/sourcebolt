// In-page resolver for the DOM fallback. Injected once per document (the
// Go side evaluates this file when window.__sbDom is undefined). It only
// reads the page and clicks or fills the controls the Go side names; it
// never sends anything anywhere.
//
// Candidate syntax (see selectors.json):
//   plain CSS        first visible match
//   text:<substr>    visible button/menuitem whose text contains substr (case-insensitive)
//   icon:<name>      visible mat-icon with that ligature text, resolved to its button
(() => {
	const PROMO = /promo|announcement|onboarding|whats-?new|welcome|tutorial|tour/i;
	const CLICKABLE = 'button,[role="button"],[role="menuitem"]';

	const visible = (el) => {
		if (!el || !el.isConnected || el.getClientRects().length === 0) return false;
		const st = getComputedStyle(el);
		return st.visibility !== "hidden" && st.display !== "none";
	};

	const label = (el) => (el.innerText || el.textContent || "").trim().toLowerCase();

	// matches returns every visible element that one candidate resolves to.
	const matches = (root, cand) => {
		root = root || document;
		if (cand.startsWith("text:")) {
			const want = cand.slice(5).trim().toLowerCase();
			return Array.from(root.querySelectorAll(CLICKABLE)).filter((el) => visible(el) && label(el).includes(want));
		}
		if (cand.startsWith("icon:")) {
			const want = cand.slice(5).trim();
			const out = [];
			for (const ic of root.querySelectorAll("mat-icon")) {
				if ((ic.textContent || "").trim() !== want) continue;
				const b = ic.closest('button,[role="button"]');
				if (b && visible(b) && !out.includes(b)) out.push(b);
			}
			return out;
		}
		try {
			return Array.from(root.querySelectorAll(cand)).filter(visible);
		} catch (e) {
			return []; // invalid CSS in an override file: skip the candidate
		}
	};

	// all returns the matches of the first candidate that matches anything.
	const all = (cands, root) => {
		for (const c of cands) {
			const m = matches(root, c);
			if (m.length) return m;
		}
		return [];
	};

	const find = (cands, root) => all(cands, root)[0] || null;

	// dismissPromos marks every promo dialog (host tag matches PROMO) with
	// data-sb-promo, which the dialog selectors exclude, and clicks its close
	// button when it has one. Returns how many were newly found.
	const dismissPromos = () => {
		let n = 0;
		for (const d of document.querySelectorAll('[role="dialog"]:not([data-sb-promo])')) {
			const hosts = [d, ...d.children].map((el) => el.tagName.toLowerCase());
			if (!hosts.some((t) => t.includes("-") && PROMO.test(t))) continue;
			d.setAttribute("data-sb-promo", "");
			n++;
			const close = find(['button[aria-label*="close" i]', 'button[aria-label*="dismiss" i]', "icon:close"], d);
			if (close) close.click();
		}
		return n;
	};

	window.__sbDom = {visible, find, all, dismissPromos};
	return true;
})()
