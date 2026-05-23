import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags as t } from "@lezer/highlight";

export const atelierLakesideLight = {
	system: "base16",
	name: "Atelier Lakeside Light",
	author: "Bram de Haan (http://atelierbramdehaan.nl)",
	variant: "light",
	palette: {
		base00: "#ebf8ff",
		base01: "#c1e4f6",
		base02: "#7ea2b4",
		base03: "#7195a8",
		base04: "#5a7b8c",
		base05: "#516d7b",
		base06: "#1f292e",
		base07: "#161b1d",
		base08: "#d22d72",
		base09: "#935c25",
		base0A: "#8a8a0f",
		base0B: "#568c3b",
		base0C: "#2d8f6f",
		base0D: "#257fad",
		base0E: "#6b6bb8",
		base0F: "#b72dd2",
	},
};

export function base16Theme(theme) {
	const p = theme.palette;
	const editor = EditorView.theme(
		{
			"&": {
				border: "1px solid var(--border)",
				borderRadius: "var(--radius-sm)",
				background: p.base00,
				color: p.base05,
				fontSize: "14px",
			},
			".cm-content": { caretColor: p.base08, fontFamily: "var(--font-mono)", minHeight: "9rem", padding: "0.65rem 0" },
			".cm-line": { padding: "0 0.75rem" },
			".cm-selectionBackground, &.cm-focused .cm-selectionBackground": { backgroundColor: p.base01 },
			".cm-cursor": { borderLeftColor: p.base08 },
			".cm-gutters": {
				background: p.base01,
				borderRight: `1px solid ${p.base02}`,
				color: p.base04,
				fontFamily: "var(--font-mono)",
			},
			".cm-activeLine": { background: "rgba(193, 228, 246, 0.45)" },
			".cm-activeLineGutter": { background: p.base01, color: p.base06 },
			".cm-matchingBracket, .cm-nonmatchingBracket": { background: p.base01, outline: `1px solid ${p.base02}` },
			"&.cm-focused": { outline: `2px solid ${p.base0D}`, outlineOffset: "2px" },
		},
		{ dark: theme.variant === "dark" },
	);

	const highlight = HighlightStyle.define([
		{ tag: t.keyword, color: p.base0E },
		{ tag: [t.name, t.deleted, t.character, t.macroName], color: p.base08 },
		{ tag: [t.propertyName, t.variableName, t.labelName], color: p.base0D },
		{ tag: [t.function(t.variableName), t.function(t.propertyName)], color: p.base0D },
		{ tag: [t.color, t.constant(t.name), t.standard(t.name)], color: p.base09 },
		{ tag: [t.definition(t.name), t.separator], color: p.base05 },
		{
			tag: [t.typeName, t.className, t.number, t.changed, t.annotation, t.modifier, t.self, t.namespace],
			color: p.base0A,
		},
		{ tag: [t.operator, t.operatorKeyword, t.url, t.escape, t.regexp, t.link], color: p.base0C },
		{ tag: [t.meta, t.comment], color: p.base03, fontStyle: "italic" },
		{ tag: t.strong, fontWeight: "700", color: p.base06 },
		{ tag: t.emphasis, fontStyle: "italic" },
		{ tag: t.strikethrough, textDecoration: "line-through" },
		{ tag: t.link, color: p.base0D, textDecoration: "underline" },
		{ tag: t.heading, color: p.base0D, fontWeight: "700" },
		{ tag: [t.atom, t.bool, t.special(t.variableName)], color: p.base09 },
		{ tag: [t.processingInstruction, t.string, t.inserted], color: p.base0B },
		{ tag: t.invalid, color: p.base00, backgroundColor: p.base08 },
	]);

	return [editor, syntaxHighlighting(highlight)];
}
