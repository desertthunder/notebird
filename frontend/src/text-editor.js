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

export function enhanceCodeMirrorComposer({ form, textarea, onChange, onClean, onReset }) {
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
				onChange(text);
			}),
		],
	});

	const mount = document.createElement("div");
	mount.className = "composer-editor composer-editor--markdown";
	textarea.insertAdjacentElement("beforebegin", mount);
	textarea.classList.add("composer__text--hidden");

	const view = new EditorView({ state, parent: mount });
	view.dom.style.setProperty("--editor-font-size", `${fontSize}px`);
	onClean();

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

	const reset = () => {
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "" } });
		onReset();
	};

	return {
		focus() {
			view.focus();
		},
		clear: reset,
	};
}
