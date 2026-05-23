import { autocompletion } from "@codemirror/autocomplete";

async function fetchSuggestions(type, q) {
	const response = await fetch(`/suggest?type=${encodeURIComponent(type)}&q=${encodeURIComponent(q)}`);
	if (!response.ok) return [];
	const data = await response.json();
	return data.items || [];
}

function wikiContext(context) {
	const before = context.matchBefore(/\[\[[^\]\n]*$/);
	if (!before) return null;
	const query = before.text.slice(2);
	return { from: before.from, to: before.to, query };
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
		if (!context.explicit && wiki.query.length < 1) return null;
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
	return autocompletion({ override: [notebirdCompletions] });
}
