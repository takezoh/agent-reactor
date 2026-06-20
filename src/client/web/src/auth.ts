export function readBearerTokenFromHash(): string {
  const hash = window.location.hash; // e.g. "#token=abc"
  if (!hash.startsWith("#")) return "";
  const params = new URLSearchParams(hash.slice(1));
  return params.get("token") ?? "";
}
