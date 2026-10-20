// In-page resolver for the DOM fallback. Injected once per document (the
// Go side evaluates this file when window.__sbDom is undefined). It only
// reads the page and clicks or fills the controls the Go side names; it
// never sends anything anywhere.
//
// Candidate syntax (see selectors.json):
//   plain CSS        first visible match
//   text:<substr>    visible button/menuitem whose text contains substr (case-insensitive)
//   icon:<name>      visible mat-icon with that ligature text, resolved to its button
//
// root arguments: undefined = the whole document; null = a scope that did
// not resolve, which matches nothing.
(() => {
	const PROMO = /promo|announcement|onboarding|whats-?new|welcome|tutorial|tour/i;
	const CLICKABLE = 'button,[role="button"],[role="menuitem"]';
	const ADD_MARK = "data-sb-add";

	const visible = (el) => {
		if (!el || !el.isConnected || el.getClientRects().length === 0) return false;
		const st = getComputedStyle(el);
		return st.visibility !== "hidden" && st.display !== "none";
	};

	const label = (el) => (el.innerText || el.textContent || "").trim().toLowerCase();

	// matches returns every visible element that one candidate resolves to.
	const matches = (root, cand) => {
		if (root === null) return [];
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

	// union returns the matches of every candidate, in candidate order.
	const union = (cands, root) => {
		const out = [];
		for (const c of cands) {
			for (const el of matches(root, c)) if (!out.includes(el)) out.push(el);
		}
		return out;
	};

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

	// addDialog returns the open add-source dialog: a sources.dialog match
	// that holds every anchor control (both source-type buttons; an input
	// or textarea alone is no proof, a share dialog has those too). It is
	// marked so later steps find the same element after its type buttons
	// give way to an input. null when none.
	// s = {dialog: [...], anchors: [[...], ...]}.
	const addDialog = (s) => {
		const marked = document.querySelector("[" + ADD_MARK + "]");
		if (marked && visible(marked)) return marked;
		for (const d of union(s.dialog)) {
			if (d.hasAttribute("data-sb-promo")) continue;
			if (s.anchors.every((c) => find(c, d))) {
				d.setAttribute(ADD_MARK, "");
				return d;
			}
		}
		return null;
	};

	// clearAdd forgets the dialog marked by an earlier add.
	const clearAdd = () => {
		for (const el of document.querySelectorAll("[" + ADD_MARK + "]")) el.removeAttribute(ADD_MARK);
		return true;
	};

	// present is all without the visibility filter for CSS candidates: answer
	// bubbles and their action bar count while scrolled away or before hover.
	const present = (cands, root) => {
		if (root === null) return [];
		for (const c of cands) {
			let m;
			if (c.startsWith("text:") || c.startsWith("icon:")) {
				m = matches(root, c);
			} else {
				try {
					m = Array.from((root || document).querySelectorAll(c));
				} catch (e) {
					m = [];
				}
			}
			if (m.length) return m;
		}
		return [];
	};

	const click = (cands, root) => {
		const el = find(cands, root);
		if (!el) return false;
		el.click();
		return true;
	};

	// setValue sets an input's value through the native setter (so framework
	// bindings see it) and dispatches input and change. No typing delays.
	const setValue = (cands, v, root) => {
		const el = find(cands, root);
		if (!el) return false;
		el.focus();
		const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
		const desc = Object.getOwnPropertyDescriptor(proto, "value");
		if (desc && desc.set) desc.set.call(el, v);
		else el.value = v;
		el.dispatchEvent(new Event("input", {bubbles: true}));
		el.dispatchEvent(new Event("change", {bubbles: true}));
		return true;
	};

	// common returns the closest element containing both a and b.
	const common = (a, b) => {
		for (let el = a; el; el = el.parentElement) if (el.contains(b)) return el;
		return null;
	};

	// chatPanel is the chat area: a chat.panel match, else the closest
	// element holding both the query box and the first answer bubble, else
	// the query box's parent. null when the query box is gone too.
	const chatPanel = (panel, input, bubble) => {
		const p = find(panel);
		if (p) return p;
		const q = present(input)[0];
		if (!q) return null;
		const b = present(bubble)[0];
		return (b && common(q, b)) || q.parentElement;
	};

	// chatState reads the answer thread: bubble count, the newest bubble's
	// text (only from chat.bubble_text elements), whether it has the done
	// marker, and the text of visible alerts inside the chat panel.
	const chatState = (bubble, text, done, alert, panel, input) => {
		const bs = present(bubble);
		const last = bs[bs.length - 1];
		let t = "", d = false;
		if (last) {
			t = present(text, last).map((e) => e.innerText || e.textContent || "").join("\n");
			d = present(done, last).length > 0;
		}
		const root = chatPanel(panel, input, bubble);
		const a = root ? all(alert, root).map((e) => (e.innerText || e.textContent || "").trim()).filter(Boolean).join("\n") : "";
		return {count: bs.length, text: t, done: d, alert: a};
	};

	window.__sbDom = {visible, find, all, union, present, dismissPromos, addDialog, clearAdd, click, setValue, chatState};
	return true;
})()
