import { baseKeymap, setBlockType, toggleMark, wrapIn } from "prosemirror-commands";
import { history, redo, undo } from "prosemirror-history";
import { keymap } from "prosemirror-keymap";
import { defaultMarkdownParser, defaultMarkdownSerializer, schema } from "prosemirror-markdown";
import { wrapInList } from "prosemirror-schema-list";
import { EditorState } from "prosemirror-state";
import { EditorView } from "prosemirror-view";

function markdownToDoc(markdown) {
	try {
		return defaultMarkdownParser.parse(markdown || "");
	} catch (error) {
		console.error("ProseMirror markdown parse failed", error);
		return schema.topNodeType.createAndFill();
	}
}

function docToMarkdown(doc) {
	try {
		return defaultMarkdownSerializer.serialize(doc);
	} catch (error) {
		console.error("ProseMirror markdown serialize failed", error);
		return "";
	}
}

function toolbarButton(label, title, run) {
	const button = document.createElement("button");
	button.type = "button";
	button.className = "composer-rich-toolbar__button";
	button.textContent = label;
	button.title = title;
	button.setAttribute("aria-label", title);
	button.addEventListener("mousedown", (event) => event.preventDefault());
	button.addEventListener("click", run);
	return button;
}

function commandButton(view, label, title, command) {
	return toolbarButton(label, title, () => {
		command(view.state, view.dispatch, view);
		view.focus();
	});
}

function linkCommand(state, dispatch) {
	const { from, to, empty } = state.selection;
	const href = window.prompt("Link URL");
	if (!href) return false;
	if (empty) {
		const text = window.prompt("Link text", href) || href;
		const mark = schema.marks.link.create({ href });
		const node = schema.text(text, [mark]);
		dispatch(state.tr.replaceSelectionWith(node, false));
		return true;
	}
	return toggleMark(schema.marks.link, { href })(state, dispatch, { from, to });
}

function buildToolbar(view) {
	const toolbar = document.createElement("div");
	toolbar.className = "composer-rich-toolbar";
	toolbar.setAttribute("role", "toolbar");
	toolbar.setAttribute("aria-label", "Rich text formatting");

	const groups = [
		[
			commandButton(view, "¶", "Paragraph", setBlockType(schema.nodes.paragraph)),
			commandButton(view, "H2", "Heading 2", setBlockType(schema.nodes.heading, { level: 2 })),
			commandButton(view, "H3", "Heading 3", setBlockType(schema.nodes.heading, { level: 3 })),
		],
		[
			commandButton(view, "B", "Bold", toggleMark(schema.marks.strong)),
			commandButton(view, "I", "Italic", toggleMark(schema.marks.em)),
			commandButton(view, "Code", "Inline code", toggleMark(schema.marks.code)),
			commandButton(view, "Link", "Link", linkCommand),
		],
		[
			commandButton(view, "• List", "Bulleted list", wrapInList(schema.nodes.bullet_list)),
			commandButton(view, "1. List", "Numbered list", wrapInList(schema.nodes.ordered_list)),
			commandButton(view, "Quote", "Block quote", wrapIn(schema.nodes.blockquote)),
			commandButton(view, "Block code", "Code block", setBlockType(schema.nodes.code_block)),
		],
		[commandButton(view, "Undo", "Undo", undo), commandButton(view, "Redo", "Redo", redo)],
	];

	for (const group of groups) {
		const wrapper = document.createElement("div");
		wrapper.className = "composer-rich-toolbar__group";
		for (const button of group) wrapper.append(button);
		toolbar.append(wrapper);
	}
	return toolbar;
}

export function enhanceProseMirrorComposer({ form, textarea, onChange, onClean, onReset }) {
	const fontSize = Math.min(24, Math.max(11, Number(form.dataset.editorFontSize) || 16));
	const shell = document.createElement("div");
	shell.className = "composer-editor composer-editor--rich";
	const mount = document.createElement("div");
	mount.className = "composer-rich-surface";
	shell.append(mount);
	textarea.insertAdjacentElement("beforebegin", shell);
	textarea.classList.add("composer__text--hidden");

	const plugins = [
		history(),
		keymap({
			"Mod-z": undo,
			"Mod-y": redo,
			"Shift-Mod-z": redo,
			"Mod-b": toggleMark(schema.marks.strong),
			"Mod-i": toggleMark(schema.marks.em),
			"Mod-`": toggleMark(schema.marks.code),
			"Mod-Enter": () => {
				form.requestSubmit();
				return true;
			},
		}),
		keymap(baseKeymap),
	];
	const state = EditorState.create({ doc: markdownToDoc(textarea.value), schema, plugins });

	const view = new EditorView(mount, {
		state,
		dispatchTransaction(transaction) {
			const nextState = view.state.apply(transaction);
			view.updateState(nextState);
			if (!transaction.docChanged) return;
			const text = docToMarkdown(nextState.doc);
			textarea.value = text;
			onChange(text);
		},
	});
	shell.prepend(buildToolbar(view));
	view.dom.style.setProperty("--editor-font-size", `${fontSize}px`);
	onClean();

	const fontInput = form.querySelector("[data-font-size]");
	if (fontInput) fontInput.value = String(fontSize);
	fontInput?.addEventListener("input", () => {
		const size = Math.min(24, Math.max(11, Number(fontInput.value) || 14));
		view.dom.style.setProperty("--editor-font-size", `${size}px`);
	});

	const reset = () => {
		const nextState = EditorState.create({ doc: markdownToDoc(""), schema, plugins });
		view.updateState(nextState);
		textarea.value = "";
		onReset();
	};

	return {
		focus() {
			view.focus();
		},
		clear: reset,
	};
}
