import { enhanceCodeMirrorComposer } from "./text-editor.js";
import { enhanceProseMirrorComposer } from "./wysiwyg.js";

const draftKey = "notebird:composer:draft";

function debounce(fn, delay) {
	let timer;
	return (...args) => {
		clearTimeout(timer);
		timer = setTimeout(() => fn(...args), delay);
	};
}

async function updatePreview(form, text) {
	const preview = form.querySelector("[data-composer-preview]");
	if (!preview) return;

	if (!text.trim()) {
		preview.innerHTML = `<p class="composer-preview__placeholder">Preview appears here as you write.</p>`;
		return;
	}

	const body = new URLSearchParams();
	body.set("text", text);

	try {
		const response = await fetch("/preview", {
			method: "POST",
			headers: { "Content-Type": "application/x-www-form-urlencoded" },
			body,
		});
		if (!response.ok) throw new Error(`Preview failed: ${response.status}`);
		preview.innerHTML = await response.text();
	} catch (error) {
		preview.innerHTML = `<p class="composer-preview__error">Preview unavailable.</p>`;
		console.error(error);
	}
}

function saveDraft(form, text) {
	if (form.dataset.draft === "off") return;
	const title = form.querySelector("input[name='title']")?.value || "";
	const tags = form.querySelector("input[name='tags']")?.value || "";
	localStorage.setItem(draftKey, JSON.stringify({ title, tags, text, updatedAt: Date.now() }));
}

function clearDraft() {
	localStorage.removeItem(draftKey);
}

function restoreDraft(form, textarea) {
	if (form.dataset.draft === "off" || textarea.value.trim()) return;
	try {
		const draft = JSON.parse(localStorage.getItem(draftKey) || "null");
		if (!draft?.text) return;
		textarea.value = draft.text;
		const title = form.querySelector("input[name='title']");
		if (title && draft.title) title.value = draft.title;
		const tags = form.querySelector("input[name='tags']");
		if (tags && draft.tags) tags.value = draft.tags;
	} catch (_) {
		clearDraft();
	}
}

function enhanceComposer(form) {
	const textarea = form.querySelector("textarea[data-editor='markdown']");
	if (!textarea || textarea.dataset.enhanced === "true") return;
	textarea.dataset.enhanced = "true";
	restoreDraft(form, textarea);

	const count = form.querySelector("[data-composer-count]");
	const updateCount = (text) => {
		if (count) count.textContent = `${text.length} chars`;
	};

	let editor;
	let initialSnapshot = "";
	const currentSnapshot = () =>
		JSON.stringify({
			title: form.querySelector("input[name='title']")?.value || "",
			tags: form.querySelector("input[name='tags']")?.value || "",
			text: textarea.value || "",
		});
	const updateDirty = () => {
		form.dataset.dirty = currentSnapshot() !== initialSnapshot ? "true" : "false";
	};
	const markClean = () => {
		initialSnapshot = currentSnapshot();
		form.dataset.dirty = "false";
	};
	const debouncedPreview = debounce((text) => updatePreview(form, text), 250);
	const debouncedDraft = debounce((text) => saveDraft(form, text), 250);
	const onTextChange = (text) => {
		updateCount(text);
		updateDirty();
		debouncedPreview(text);
		debouncedDraft(text);
	};
	const onReset = () => {
		updateCount("");
		markClean();
		clearDraft();
		updatePreview(form, "");
	};

	const enhancer = form.dataset.editorMode === "wysiwyg" ? enhanceProseMirrorComposer : enhanceCodeMirrorComposer;
	editor = enhancer({ form, textarea, onChange: onTextChange, onClean: markClean, onReset });

	form
		.querySelectorAll("input[name='title'], input[name='tags']")
		.forEach((input) => input.addEventListener("input", updateDirty));
	form.addEventListener("submit", () => {
		markClean();
		if (form.dataset.draft !== "off") clearDraft();
	});
	form.addEventListener("reset", () => editor.clear());
	window.addEventListener("notebird:chirp-created", () => {
		editor.clear();
		collapseComposerIfClean();
	});

	updateCount(textarea.value);
	updatePreview(form, textarea.value);
}

function enhanceComposerToggle(section) {
	const button = section.querySelector("[data-toggle-composer]");
	const form = section.querySelector("form[data-composer]");
	if (!button || !form || button.dataset.enhanced === "true") return;
	button.dataset.enhanced = "true";
	button.addEventListener("click", () => {
		const collapsed = form.hidden;
		form.hidden = !collapsed;
		section.classList.toggle("is-collapsed", !collapsed);
		button.setAttribute("aria-expanded", String(collapsed));
		button.textContent = collapsed ? "Collapse" : "Compose";
		if (collapsed) requestAnimationFrame(() => section.querySelector(".cm-content, .ProseMirror")?.focus());
	});
}

function expandComposer({ focus = false } = {}) {
	const section = document.querySelector("#composer");
	const form = section?.querySelector("form[data-composer]");
	const button = section?.querySelector("[data-toggle-composer]");
	if (!section || !form) return;
	form.hidden = false;
	section.classList.remove("is-collapsed");
	button?.setAttribute("aria-expanded", "true");
	if (button) button.textContent = "Collapse";
	if (focus) requestAnimationFrame(() => section.querySelector(".cm-content, .ProseMirror")?.focus());
}

function collapseComposerIfClean() {
	const section = document.querySelector("#composer");
	const form = section?.querySelector("form[data-composer]");
	const button = section?.querySelector("[data-toggle-composer]");
	if (!section || !form || form.dataset.dirty === "true") return;
	form.hidden = true;
	section.classList.add("is-collapsed");
	button?.setAttribute("aria-expanded", "false");
	if (button) button.textContent = "Compose";
}

function enhanceAllComposers() {
	document.querySelectorAll("#composer").forEach(enhanceComposerToggle);
	document.querySelectorAll("form[data-composer]").forEach(enhanceComposer);
}

function markActiveChirp(id) {
	document.querySelectorAll(".chirp.is-active").forEach((chirp) => chirp.classList.remove("is-active"));
	if (!id) return;
	document.querySelector(`[data-chirp-id="${CSS.escape(id)}"]`)?.classList.add("is-active");
}

function syncActiveChirpFromDetail() {
	markActiveChirp(document.querySelector("#detail")?.dataset.selectedChirpId || "");
}

function installActiveChirpTracking() {
	document.body.addEventListener("click", (event) => {
		const chirp = event.target.closest?.("[data-chirp-id]");
		if (chirp) markActiveChirp(chirp.dataset.chirpId);
	});
	document.body.addEventListener("htmx:afterSwap", syncActiveChirpFromDetail);
	syncActiveChirpFromDetail();
}

function dirtyComposer() {
	return document.querySelector("form[data-composer][data-dirty='true']");
}

function confirmDirtyComposer(eventSource) {
	const form = dirtyComposer();
	if (!form) return true;
	if (
		eventSource === form ||
		eventSource?.closest?.("form[data-composer]") === form ||
		eventSource?.matches?.("[type='submit']")
	)
		return true;
	return window.confirm("Discard unsaved changes in the composer?");
}

function installDirtyComposerGuards() {
	window.addEventListener("beforeunload", (event) => {
		if (!dirtyComposer()) return;
		event.preventDefault();
		event.returnValue = "";
	});
	document.body.addEventListener("htmx:beforeRequest", (event) => {
		if (confirmDirtyComposer(event.detail.elt)) return;
		event.preventDefault();
	});
}

// TODO: these should be handled by the backend
function showNotification({ type = "success", title = "Saved", message = "Done." } = {}) {
	const host = document.querySelector("#notifications");
	if (!host) return;
	const item = document.createElement("div");
	item.className = `notification notification--${type}`;
	item.setAttribute("role", type === "error" ? "alert" : "status");
	item.innerHTML = `
		<span class="notification__icon" aria-hidden="true">${type === "error" ? "!" : "✓"}</span>
		<span><strong class="notification__title"></strong><span class="notification__message"></span></span>
		<button class="notification__close" type="button" aria-label="Dismiss">×</button>
	`;
	item.querySelector(".notification__title").textContent = title;
	item.querySelector(".notification__message").textContent = message ? ` ${message}` : "";
	const remove = () => {
		item.classList.remove("is-visible");
		setTimeout(() => item.remove(), 180);
	};
	item.querySelector(".notification__close")?.addEventListener("click", remove);
	host.append(item);
	requestAnimationFrame(() => item.classList.add("is-visible"));
	setTimeout(remove, type === "error" ? 7000 : 3600);
}

function installNotifications() {
	document.body.addEventListener("notebird:notice", (event) => {
		showNotification(event.detail || { type: "success", title: "Done", message: "" });
	});
	document.body.addEventListener("htmx:responseError", (event) => {
		if (event.detail.xhr?.getResponseHeader("HX-Trigger")?.includes("notebird:notice")) return;
		showNotification({
			type: "error",
			title: "Request failed",
			message: event.detail.xhr?.responseText?.trim() || "The server returned an error.",
		});
	});
	document.body.addEventListener("htmx:sendError", () =>
		showNotification({ type: "error", title: "Network error", message: "Could not reach Notebird." }),
	);
	document.body.addEventListener("htmx:timeout", () =>
		showNotification({ type: "error", title: "Request timed out", message: "Try again in a moment." }),
	);
}

function installGlobalShortcuts() {
	document.addEventListener("keydown", (event) => {
		if (event.defaultPrevented) return;
		const target = event.target;
		const typing = target?.matches?.("input, textarea, [contenteditable='true'], .cm-content, .ProseMirror");
		if (event.key === "/" && !typing) {
			event.preventDefault();
			document.querySelector("#search")?.focus();
		}
		if (event.key.toLowerCase() === "n" && !typing) {
			event.preventDefault();
			expandComposer({ focus: true });
		}
	});
}

document.addEventListener("DOMContentLoaded", () => {
	enhanceAllComposers();
	installActiveChirpTracking();
	installGlobalShortcuts();
	installDirtyComposerGuards();
	installNotifications();
});
document.body.addEventListener("htmx:afterSwap", enhanceAllComposers);
