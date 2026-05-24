import { indentWithTab } from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { Compartment, EditorState } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { basicSetup, EditorView } from "codemirror";
import { atelierLakesideLight, base16Theme } from "./base16.js";
import { notebirdAutocomplete } from "./suggestions.js";

const markdownConfig = new Compartment();
const wrapConfig = new Compartment();
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
	let wrapEnabled = form.dataset.wordWrap !== "false";
	const fontSize = Math.min(24, Math.max(11, Number(form.dataset.editorFontSize) || 16));
	const state = EditorState.create({
		doc: textarea.value,
		extensions: [
			basicSetup,
			keymap.of([
				indentWithTab,
				{
					key: "Mod-Enter",
					run: () => {
						form.requestSubmit();
						return true;
					},
				},
			]),
			markdownConfig.of(markdown({ codeLanguages: languages })),
			notebirdAutocomplete(),
			base16Theme(atelierLakesideLight),
			wrapConfig.of(wrapEnabled ? EditorView.lineWrapping : []),
			EditorView.updateListener.of((update) => {
				if (!update.docChanged) return;
				const text = update.state.doc.toString();
				textarea.value = text;
				updateCount(text);
				updateDirty();
				debouncedPreview(text);
				debouncedDraft(text);
			}),
		],
	});

	const mount = document.createElement("div");
	mount.className = "composer-editor";
	textarea.insertAdjacentElement("beforebegin", mount);
	textarea.classList.add("composer__text--hidden");

	const view = new EditorView({ state, parent: mount });
	view.dom.style.setProperty("--editor-font-size", `${fontSize}px`);
	markClean();
	form
		.querySelectorAll("input[name='title'], input[name='tags']")
		.forEach((input) => input.addEventListener("input", updateDirty));
	const wrapButton = form.querySelector("[data-toggle-wrap]");
	if (wrapButton) wrapButton.textContent = wrapEnabled ? "Wrap on" : "Wrap off";
	wrapButton?.addEventListener("click", () => {
		wrapEnabled = !wrapEnabled;
		view.dispatch({ effects: wrapConfig.reconfigure(wrapEnabled ? EditorView.lineWrapping : []) });
		wrapButton.textContent = wrapEnabled ? "Wrap on" : "Wrap off";
	});
	const fontInput = form.querySelector("[data-font-size]");
	if (fontInput) fontInput.value = String(fontSize);
	fontInput?.addEventListener("input", () => {
		const size = Math.min(24, Math.max(11, Number(fontInput.value) || 14));
		view.dom.style.setProperty("--editor-font-size", `${size}px`);
	});
	form.addEventListener("submit", () => {
		markClean();
		if (form.dataset.draft !== "off") clearDraft();
	});
	form.addEventListener("reset", () => {
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "" } });
		updateCount("");
		markClean();
		clearDraft();
		updatePreview(form, "");
	});
	window.addEventListener("notebird:chirp-created", () => {
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "" } });
		updateCount("");
		markClean();
		clearDraft();
		updatePreview(form, "");
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
		if (collapsed) requestAnimationFrame(() => section.querySelector(".cm-content")?.focus());
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
	if (focus) requestAnimationFrame(() => section.querySelector(".cm-content")?.focus());
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

function installGlobalShortcuts() {
	document.addEventListener("keydown", (event) => {
		if (event.defaultPrevented) return;
		const target = event.target;
		const typing = target?.matches?.("input, textarea, [contenteditable='true'], .cm-content");
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
});
document.body.addEventListener("htmx:afterSwap", enhanceAllComposers);
