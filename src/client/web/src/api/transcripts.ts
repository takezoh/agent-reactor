export type TranscriptKindParam = "transcript" | "event-log";

export type FetchTranscriptDeps = {
  fetchFn?: typeof fetch;
  bearerToken: string;
};

export type FetchTranscriptResult = {
  status: "ok" | "not-modified" | "empty";
  // status === "ok" のときのみ意味あり
  text: string;
  // 応答の ETag(あれば)。client は次回呼び出しで `If-None-Match` として送る
  etag?: string;
  // 次の offset として使うべき byte 位置(現 offset + text の byte 長)。
  // text を UTF-8 として bytes に encode してカウント。
  nextOffset: number;
};

export async function fetchSessionFile(
  sessionId: string,
  kind: TranscriptKindParam,
  offset: number,
  deps: FetchTranscriptDeps,
  prevEtag?: string,
): Promise<FetchTranscriptResult> {
  const url = `/api/sessions/${encodeURIComponent(sessionId)}/${kind}?offset=${offset}`;

  const headers: Record<string, string> = {
    Authorization: `Bearer ${deps.bearerToken}`,
  };
  if (prevEtag) {
    headers["If-None-Match"] = prevEtag;
  }

  const r = await (deps.fetchFn ?? fetch)(url, { headers });

  if (r.status === 304) {
    return { status: "not-modified", text: "", etag: prevEtag, nextOffset: offset };
  }

  if (r.status === 204) {
    return {
      status: "empty",
      text: "",
      etag: r.headers.get("ETag") ?? undefined,
      nextOffset: offset,
    };
  }

  if (r.status === 200) {
    const text = await r.text();
    const bytes = new TextEncoder().encode(text).byteLength;
    return {
      status: "ok",
      text,
      etag: r.headers.get("ETag") ?? undefined,
      nextOffset: offset + bytes,
    };
  }

  if (r.status >= 400) {
    throw new Error(`fetchSessionFile failed: ${r.status}`);
  }

  throw new Error(`fetchSessionFile unexpected status: ${r.status}`);
}

export function splitLines(text: string): string[] {
  if (text === "") return [];
  // trailing \n を保持しない: ["foo", "bar"]
  const out = text.split("\n");
  if (out.length > 0 && out[out.length - 1] === "") out.pop();
  return out;
}
