import { describe, expect, it, vi } from "vitest";
import { fetchSessionFile, splitLines } from "./transcripts";

function makeResponse(
  body: string | null,
  status: number,
  headers: Record<string, string> = {},
): Response {
  return new Response(body, { status, headers });
}

describe("fetchSessionFile", () => {
  it("TestFetchOk_ReturnsTextAndEtag", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("a\nb\n", 200, { ETag: "abc" }));
    const result = await fetchSessionFile("sess1", "transcript", 0, {
      fetchFn,
      bearerToken: "tok",
    });
    expect(result.status).toBe("ok");
    expect(result.text).toBe("a\nb\n");
    expect(result.etag).toBe("abc");
    // "a\nb\n" in UTF-8 is 4 bytes
    expect(result.nextOffset).toBe(4);
  });

  it("TestFetchOk_BytesNotChars", async () => {
    // "あ\n" — "あ" is 3 bytes in UTF-8, "\n" is 1 byte = 4 bytes total
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("あ\n", 200, { ETag: "xyz" }));
    const result = await fetchSessionFile("sess1", "transcript", 0, {
      fetchFn,
      bearerToken: "tok",
    });
    expect(result.status).toBe("ok");
    expect(result.nextOffset).toBe(4);
  });

  it("TestFetch304_ReturnsNotModified", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse(null, 304));
    const result = await fetchSessionFile(
      "sess1",
      "transcript",
      10,
      {
        fetchFn,
        bearerToken: "tok",
      },
      "prev",
    );
    // should have sent If-None-Match
    const [, options] = fetchFn.mock.calls[0] as [
      string,
      RequestInit & { headers: Record<string, string> },
    ];
    expect(options.headers["If-None-Match"]).toBe("prev");
    expect(result.status).toBe("not-modified");
    expect(result.etag).toBe("prev");
    expect(result.nextOffset).toBe(10);
  });

  it("TestFetch204_Empty", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse(null, 204));
    const result = await fetchSessionFile("sess1", "event-log", 5, {
      fetchFn,
      bearerToken: "tok",
    });
    expect(result.status).toBe("empty");
    expect(result.nextOffset).toBe(5);
  });

  it("TestFetch4xx_Throws", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse(null, 400));
    await expect(
      fetchSessionFile("sess1", "transcript", 0, { fetchFn, bearerToken: "tok" }),
    ).rejects.toThrow("fetchSessionFile failed: 400");
  });

  it("TestFetch404_Throws", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse(null, 404));
    await expect(
      fetchSessionFile("sess1", "transcript", 0, { fetchFn, bearerToken: "tok" }),
    ).rejects.toThrow("fetchSessionFile failed: 404");
  });

  it("TestFetchSendsBearer", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("data", 200));
    await fetchSessionFile("sess1", "transcript", 0, { fetchFn, bearerToken: "xxx" });
    const [, options] = fetchFn.mock.calls[0] as [
      string,
      RequestInit & { headers: Record<string, string> },
    ];
    expect(options.headers.Authorization).toBe("Bearer xxx");
  });
});

describe("splitLines", () => {
  it("TestSplitLines_Empty", () => {
    expect(splitLines("")).toEqual([]);
  });

  it("TestSplitLines_TrailingNewlineStripped", () => {
    expect(splitLines("a\nb\n")).toEqual(["a", "b"]);
  });

  it("TestSplitLines_NoTrailingNewline", () => {
    expect(splitLines("a\nb")).toEqual(["a", "b"]);
  });
});
