import { indentWithTab } from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { Compartment, EditorState } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { basicSetup, EditorView } from "codemirror";
import { atelierLakesideLight, base16Theme } from "./base16.js";

const markdownConfig = new Compartment();

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

function enhanceComposer(form) {
	const textarea = form.querySelector("textarea[data-editor='markdown']");
	if (!textarea || textarea.dataset.enhanced === "true") return;
	textarea.dataset.enhanced = "true";

	const syncAlpine = () => {
		textarea.dispatchEvent(new Event("input", { bubbles: true }));
	};

	const debouncedPreview = debounce((text) => updatePreview(form, text), 250);
	const state = EditorState.create({
		doc: textarea.value,
		extensions: [
			basicSetup,
			keymap.of([indentWithTab]),
			markdownConfig.of(markdown({ codeLanguages: languages })),
			base16Theme(atelierLakesideLight),
			EditorView.lineWrapping,
			EditorView.updateListener.of((update) => {
				if (!update.docChanged) return;
				const text = update.state.doc.toString();
				textarea.value = text;
				syncAlpine();
				debouncedPreview(text);
			}),
		],
	});

	const mount = document.createElement("div");
	mount.className = "composer-editor";
	textarea.insertAdjacentElement("beforebegin", mount);
	textarea.classList.add("composer__text--hidden");

	const view = new EditorView({ state, parent: mount });
	form.addEventListener("reset", () => {
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "" } });
		updatePreview(form, "");
	});
	window.addEventListener("notebird:chirp-created", () => {
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "" } });
		updatePreview(form, "");
	});

	updatePreview(form, textarea.value);
}

function enhanceAllComposers() {
	document.querySelectorAll("form[data-composer]").forEach(enhanceComposer);
}

document.addEventListener("DOMContentLoaded", enhanceAllComposers);
document.body.addEventListener("htmx:afterSwap", enhanceAllComposers);
