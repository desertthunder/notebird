import { indentWithTab } from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { Compartment, EditorState } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { basicSetup, EditorView } from "codemirror";
import { atelierLakesideLight, base16Theme } from "./base16.js";
import { notebirdAutocomplete } from "./suggestions.js";

const markdownConfig = new Compartment();
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
	localStorage.setItem(draftKey, JSON.stringify({ title, text, updatedAt: Date.now() }));
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
	} catch (_) {
		clearDraft();
	}
}

function enhanceComposer(form) {
	const textarea = form.querySelector("textarea[data-editor='markdown']");
	if (!textarea || textarea.dataset.enhanced === "true") return;
	textarea.dataset.enhanced = "true";
	restoreDraft(form, textarea);

	const syncAlpine = () => {
		textarea.dispatchEvent(new Event("input", { bubbles: true }));
	};

	const debouncedPreview = debounce((text) => updatePreview(form, text), 250);
	const debouncedDraft = debounce((text) => saveDraft(form, text), 250);
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
			EditorView.lineWrapping,
			EditorView.updateListener.of((update) => {
				if (!update.docChanged) return;
				const text = update.state.doc.toString();
				textarea.value = text;
				syncAlpine();
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
	form.addEventListener("submit", () => {
		if (form.dataset.draft !== "off") clearDraft();
	});
	form.addEventListener("reset", () => {
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "" } });
		clearDraft();
		updatePreview(form, "");
	});
	window.addEventListener("notebird:chirp-created", () => {
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "" } });
		clearDraft();
		updatePreview(form, "");
	});

	updatePreview(form, textarea.value);
}

function enhanceAllComposers() {
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
			document.querySelector(".cm-content")?.focus();
		}
	});
}

document.addEventListener("DOMContentLoaded", () => {
	enhanceAllComposers();
	installActiveChirpTracking();
	installGlobalShortcuts();
});
document.body.addEventListener("htmx:afterSwap", enhanceAllComposers);
