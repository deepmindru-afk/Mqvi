/**
 * Help center manifest — the language-agnostic structure (single source of truth).
 *
 * Categories and article order live here; translations live elsewhere:
 *  - article/category TITLES → i18n "help" namespace (loaded with the app, used in the nav)
 *  - article BODIES → help/content/{lang}/{slug}.md (lazy-loaded on open, EN fallback)
 *
 * Adding a language never touches this file. Adding an article = add a slug here +
 * a title key in help.json + an EN markdown file (translations follow over time).
 */

export type HelpIcon =
  | "rocket" | "chat" | "voice" | "friends" | "servers"
  | "roles" | "lock" | "user" | "sliders" | "server" | "sparkle";

export type HelpArticleRef = {
  /** Matches help/content/{lang}/<slug>.md */
  slug: string;
  /** i18n key in the "help" namespace */
  titleKey: string;
};

export type HelpCategory = {
  id: string;
  /** i18n key in the "help" namespace */
  labelKey: string;
  icon: HelpIcon;
  articles: HelpArticleRef[];
};

export const HELP_CATEGORIES: HelpCategory[] = [
  {
    id: "getting-started",
    labelKey: "cat_gettingStarted",
    icon: "rocket",
    articles: [
      { slug: "welcome", titleKey: "art_welcome" },
      { slug: "register-login", titleKey: "art_register-login" },
    ],
  },
  {
    id: "messaging",
    labelKey: "cat_messaging",
    icon: "chat",
    articles: [{ slug: "send-messages", titleKey: "art_send-messages" }],
  },
  { id: "voice", labelKey: "cat_voice", icon: "voice", articles: [] },
  { id: "friends", labelKey: "cat_friends", icon: "friends", articles: [] },
  { id: "servers", labelKey: "cat_servers", icon: "servers", articles: [] },
  { id: "roles", labelKey: "cat_roles", icon: "roles", articles: [] },
  { id: "privacy", labelKey: "cat_privacy", icon: "lock", articles: [] },
  { id: "account", labelKey: "cat_account", icon: "user", articles: [] },
  { id: "app", labelKey: "cat_app", icon: "sliders", articles: [] },
  { id: "selfhost", labelKey: "cat_selfhost", icon: "server", articles: [] },
  { id: "nice-to-know", labelKey: "cat_niceToKnow", icon: "sparkle", articles: [] },
];
