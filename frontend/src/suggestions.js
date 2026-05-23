import { autocompletion } from "@codemirror/autocomplete";

async function fetchSuggestions(type, q) {
	const response = await fetch(`/suggest?type=${encodeURIComponent(type)}&q=${encodeURIComponent(q)}`);
	if (!response.ok) return [];
	const data = await response.json();
	return data.items || [];
}

function wikiContext(context) {
	const line = context.state.doc.lineAt(context.pos);
	const before = line.text.slice(0, context.pos - line.from);
	const after = line.text.slice(context.pos - line.from);
	const start = before.lastIndexOf("[[");
	if (start < 0) return null;
	const closeBeforeCursor = before.lastIndexOf("]]");
	if (closeBeforeCursor > start) return null;
	let query = before.slice(start + 2);
	const closeAfterCursor = after.indexOf("]]");
	const to = closeAfterCursor === 0 ? context.pos + 2 : context.pos;
	return { from: line.from + start, to, query };
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
			options: items.map((item) => ({
				label: item.label,
				detail: item.detail,
				type: "text",
				apply: `[[${item.value}]]`,
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
	return autocompletion({ override: [notebirdCompletions], activateOnTyping: true });
}
