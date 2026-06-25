// Dependency-free fuzzy ranker — sahilm/fuzzy compatible enough for the
// command palette's ToolSelectPhase (FR-008, ADR-0038).
// Returns hits in score-desc, input-order-stable order with absolute-index
// match ranges on the original (case-preserving) text.

export type FuzzyRange = [number, number]; // [start, endExclusive]

export interface FuzzyHit<T> {
  item: T;
  score: number;
  ranges: FuzzyRange[];
}

// Bonus scheme mirrors sahilm/fuzzy: leading match + consecutive run + separator boundary.
const BONUS_LEADING = 10;
const BONUS_CONSECUTIVE = 5;
const BONUS_BOUNDARY = 3;

function isBoundary(prev: string | undefined): boolean {
  if (prev === undefined) return true;
  return /[\s_\-./\\]/.test(prev);
}

function scoreText(text: string, query: string): { score: number; ranges: FuzzyRange[] } | null {
  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase();
  const indices: number[] = [];
  let ti = 0;
  for (const qc of lowerQuery) {
    while (ti < lowerText.length && lowerText[ti] !== qc) ti++;
    if (ti >= lowerText.length) return null;
    indices.push(ti);
    ti++;
  }
  let score = 0;
  let prevIdx = -2;
  for (const idx of indices) {
    score += 1;
    if (idx === 0) score += BONUS_LEADING;
    else if (isBoundary(text[idx - 1])) score += BONUS_BOUNDARY;
    if (idx === prevIdx + 1) score += BONUS_CONSECUTIVE;
    prevIdx = idx;
  }
  // Merge adjacent single-char matches into half-open ranges.
  const ranges: FuzzyRange[] = [];
  for (const idx of indices) {
    const last = ranges[ranges.length - 1];
    if (last && last[1] === idx) last[1] = idx + 1;
    else ranges.push([idx, idx + 1]);
  }
  return { score, ranges };
}

export function fuzzyRank<T>(
  items: T[],
  query: string,
  getText: (item: T) => string,
): FuzzyHit<T>[] {
  if (query === "") return items.map((item) => ({ item, score: 0, ranges: [] }));
  const hits: Array<FuzzyHit<T> & { order: number }> = [];
  items.forEach((item, i) => {
    const r = scoreText(getText(item), query);
    if (r) hits.push({ item, score: r.score, ranges: r.ranges, order: i });
  });
  hits.sort((a, b) => b.score - a.score || a.order - b.order);
  return hits.map(({ item, score, ranges }) => ({ item, score, ranges }));
}
