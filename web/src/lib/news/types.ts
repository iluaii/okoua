// SPDX-License-Identifier: AGPL-3.0-or-later
// Regional news, as served by GET /api/news/{uid} and GET /api/news-item?id= (see server/internal/news).

export type SourceKind = 'official' | 'energy' | 'media';

export interface NewsItem {
	/** "<channel>/<number>" */
	id: string;
	region: number;
	/** First line of the post. */
	title: string;
	/** Beginning of the rest of the post; the full text stays at the source. */
	text: string;
	/** Link to the original post. */
	url: string;
	source: { name: string; kind: SourceKind };
	publishedAt: string;
}

/** A reply from POST /api/ai/news. */
export interface AiAnswer {
	question: string;
	answer: string;
	questions: string[];
	model: string;
	cached: boolean;
	/** People reported this answer: it is withheld, read the original instead. */
	hidden?: boolean;
}
