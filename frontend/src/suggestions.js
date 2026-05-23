import { autocompletion } from "@codemirror/autocomplete";
import { Prec } from "@codemirror/state";

async function fetchSuggestions(type, q) {
	const response = await fetch(`/suggest?type=${encodeURIComponent(type)}&q=${encodeURIComponent(q)}`);
	if (!response.ok) return [];
	const data = await response.json();
	return data.items || [];
}

function wikiContext(context) {
	const text = context.state.doc.sliceString(0, context.pos);
	const start = text.lastIndexOf("[[");
	if (start < 0) return null;
	const closeBeforeCursor = text.lastIndexOf("]]");
	if (closeBeforeCursor > start) return null;
	const query = text.slice(start + 2);
	if (/\n\s*\n/.test(query)) return null;
	const after = context.state.doc.sliceString(context.pos, Math.min(context.state.doc.length, context.pos + 2));
	return { from: start + 2, to: context.pos, query, hasClose: after === "]]" };
}

function tagContext(context) {
	const before = context.matchBefore(/(^|\s)#[\w-]*$/);
	if (!before) return null;
	const hashIndex = before.text.lastIndexOf("#");
	if (hashIndex < 0) return null;
	const from = before.from + hashIndex;
	return { from, to: before.to, query: before.text.slice(hashIndex + 1) };
}

async function notebirdCompletions(context) {
	const wiki = wikiContext(context);
	if (wiki) {
		const items = await fetchSuggestions("chirp", wiki.query);
		return {
			from: wiki.from,
			to: wiki.to,
			validFor: /^[^\]\n]*$/,
			options: items.map((item) => ({
				label: item.label,
				detail: item.detail,
				type: "text",
				apply: wiki.hasClose ? item.value : `${item.value}]]`,
			})),
		};
	}

	const tag = tagContext(context);
	if (tag) {
		if (!context.explicit && tag.query.length < 1) return null;
		const items = await fetchSuggestions("tag", tag.query);
		return {
			from: tag.from,
			to: tag.to,
			options: items.map((item) => ({ label: item.label, detail: item.detail, type: "keyword", apply: item.value })),
		};
	}
	return null;
}

export function notebirdAutocomplete() {
	return Prec.highest(autocompletion({ override: [notebirdCompletions], activateOnTyping: true }));
}
