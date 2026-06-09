/**
 * Help article body loader — lazy, per-language, with English fallback.
 *
 * Globs every language's markdown but as lazy loaders, so only the chunk for the
 * slug actually opened (in the active language) is fetched. If the active language
 * has no translation for a slug, the English body is loaded instead.
 */

const loaders = import.meta.glob("./content/*/*.md", {
  query: "?raw",
  import: "default",
}) as Record<string, () => Promise<string>>;

const keyFor = (lang: string, slug: string) => `./content/${lang}/${slug}.md`;

export type LoadedArticle = { slug: string; body: string; translated: boolean };

/** Loads a slug's markdown body for `lang`, falling back to English. Null if neither exists. */
export async function loadArticleBody(lang: string, slug: string): Promise<LoadedArticle | null> {
  const active = loaders[keyFor(lang, slug)];
  if (active) return { slug, body: await active(), translated: true };

  const english = loaders[keyFor("en", slug)];
  if (english) return { slug, body: await english(), translated: lang === "en" };

  return null;
}
