(() => {
  "use strict";

  const I18N = (window.SERAPH_I18N = window.SERAPH_I18N || { en: {} });
  const DATA_URL = "data/state.json";
  const REFRESH_MS = 60_000;
  const STORAGE = {
    lang: "seraph.lang.v1",
    region: "seraph.region.v1",
    favorites: "seraph.favorites.v1",
    view: "seraph.view.v1",
  };
  const CATEGORY_ORDER = [
    "banking",
    "payments",
    "telecom",
    "messaging",
    "social",
    "cloud",
    "streaming",
    "gaming",
    "transport",
    "ecommerce",
    "gov",
  ];
  const STATUS_RANK = { major: 0, degraded: 1, unknown: 2, operational: 3 };

  const state = {
    lang: "en",
    region: "global",
    view: "all",
    query: "",
    favorites: new Set(),
    snapshot: null,
    timer: null,
  };

  const els = {
    langToggle: document.getElementById("lang-toggle"),
    search: document.getElementById("search"),
    regions: document.getElementById("regions"),
    summary: document.getElementById("summary"),
    updated: document.getElementById("updated"),
    banner: document.getElementById("banner"),
    bannerText: document.getElementById("banner-text"),
    retry: document.getElementById("retry"),
    groups: document.getElementById("groups"),
    empty: document.getElementById("empty"),
  };

  function readStore(key) {
    try {
      return window.localStorage.getItem(key);
    } catch {
      return null;
    }
  }

  function writeStore(key, value) {
    try {
      window.localStorage.setItem(key, value);
    } catch {
      /* storage may be unavailable in private mode */
    }
  }

  function loadFavorites() {
    try {
      const parsed = JSON.parse(readStore(STORAGE.favorites) || "[]");
      return new Set(Array.isArray(parsed) ? parsed.filter((item) => typeof item === "string") : []);
    } catch {
      return new Set();
    }
  }

  function t(key, vars) {
    const dict = I18N[state.lang] || I18N.en || {};
    const fallback = I18N.en || {};
    let text = dict[key] ?? fallback[key] ?? key;

    if (vars) {
      for (const [name, value] of Object.entries(vars)) {
        text = text.replaceAll(`{${name}}`, String(value));
      }
    }

    return text;
  }

  function detectLanguage() {
    const saved = readStore(STORAGE.lang);
    if (saved === "en" || saved === "pt-br") {
      return saved;
    }

    return (navigator.language || "en").toLowerCase().startsWith("pt") ? "pt-br" : "en";
  }

  function regionLabel(code) {
    if (code === "global") {
      return t("regionGlobal");
    }

    if (code === "BR") {
      return t("regionBR");
    }

    try {
      const display = new Intl.DisplayNames([state.lang === "pt-br" ? "pt-BR" : "en"], { type: "region" });
      return display.of(code) || code;
    } catch {
      return code;
    }
  }

  function statusLabel(status) {
    const key = `status_${status}`;
    const dict = I18N[state.lang] || {};

    return dict[key] ? t(key) : t("status_unknown");
  }

  function categoryLabel(category) {
    return t(`category_${category}`);
  }

  function applyStaticTranslations() {
    document.documentElement.lang = state.lang;

    for (const node of document.querySelectorAll("[data-i18n]")) {
      node.textContent = t(node.dataset.i18n);
    }

    for (const node of document.querySelectorAll("[data-i18n-placeholder]")) {
      node.placeholder = t(node.dataset.i18nPlaceholder);
    }

    for (const node of document.querySelectorAll("[data-i18n-aria]")) {
      node.setAttribute("aria-label", t(node.dataset.i18nAria));
    }

    els.langToggle.textContent = state.lang === "en" ? "PT-BR" : "EN";
    els.bannerText.textContent = t("error");
    els.search.value = state.query;
  }

  function availableRegions() {
    const regions = new Set(["global"]);

    for (const service of state.snapshot?.services || []) {
      for (const region of service.regions || []) {
        regions.add(region);
      }
    }

    return [...regions].sort((a, b) => {
      if (a === "global") return -1;
      if (b === "global") return 1;
      return a.localeCompare(b);
    });
  }

  function renderRegions() {
    els.regions.replaceChildren();

    const regions = availableRegions();
    if (!regions.includes(state.region)) {
      state.region = "global";
    }

    for (const region of regions) {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = regionLabel(region);
      button.dataset.region = region;
      button.setAttribute("aria-pressed", String(region === state.region));
      els.regions.append(button);
    }
  }

  function matchesFilters(service) {
    if (!(service.regions || []).includes(state.region)) {
      return false;
    }

    if (state.view === "favorites" && !state.favorites.has(service.slug)) {
      return false;
    }

    const query = state.query.trim().toLowerCase();
    if (!query) {
      return true;
    }

    const haystack = `${service.name} ${service.slug} ${categoryLabel(service.category)}`.toLowerCase();
    return haystack.includes(query);
  }

  function formatTime(value) {
    if (!value) {
      return t("never");
    }

    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return t("never");
    }

    return new Intl.DateTimeFormat(state.lang === "pt-br" ? "pt-BR" : "en", {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(date);
  }

  function buildCard(service) {
    const article = document.createElement("article");
    article.className = "card";
    article.dataset.status = service.status || "unknown";

    const header = document.createElement("header");

    const title = document.createElement("h3");
    if (service.official_url) {
      const link = document.createElement("a");
      link.href = service.official_url;
      link.target = "_blank";
      link.rel = "noopener noreferrer";
      link.textContent = service.name;
      title.append(link);
    } else {
      title.textContent = service.name;
    }

    const star = document.createElement("button");
    star.type = "button";
    star.className = "star";
    star.dataset.slug = service.slug;
    const favorite = state.favorites.has(service.slug);
    star.setAttribute("aria-pressed", String(favorite));
    star.setAttribute("aria-label", t(favorite ? "removeFavorite" : "addFavorite", { name: service.name }));
    star.textContent = favorite ? "★" : "☆";

    header.append(title, star);

    const status = document.createElement("p");
    status.className = "status";
    const dot = document.createElement("span");
    dot.className = "dot";
    dot.setAttribute("aria-hidden", "true");
    status.append(dot, document.createTextNode(statusLabel(service.status)));

    const details = document.createElement("p");
    details.className = "details";

    const parts = [];
    if (typeof service.latency_ms === "number" && service.latency_ms > 0) {
      parts.push(`${t("latency")}: ${service.latency_ms} ms`);
    }

    parts.push(`${t("checked")}: ${formatTime(service.observed_at)}`);
    details.textContent = parts.join(" · ");

    const meta = document.createElement("p");
    meta.className = "category";
    meta.textContent = categoryLabel(service.category);

    article.append(header, meta, status, details);

    if (service.error) {
      const error = document.createElement("p");
      error.className = "note";
      error.textContent = service.error;
      article.append(error);
    }

    if (Array.isArray(service.points) && service.points.length > 0) {
      const history = document.createElement("div");
      history.className = "history";
      history.setAttribute("aria-hidden", "true");

      for (const point of service.points.slice(-30)) {
        const cell = document.createElement("span");
        cell.dataset.status = point.status || "unknown";
        history.append(cell);
      }

      article.append(history);
    }

    return article;
  }

  function renderGroups() {
    els.groups.replaceChildren();

    const services = (state.snapshot?.services || []).filter(matchesFilters);
    const grouped = new Map();

    for (const service of services) {
      const key = service.category || "other";

      if (!grouped.has(key)) {
        grouped.set(key, []);
      }

      grouped.get(key).push(service);
    }

    const order = [...CATEGORY_ORDER.filter((category) => grouped.has(category))];
    for (const category of grouped.keys()) {
      if (!order.includes(category)) {
        order.push(category);
      }
    }

    for (const category of order) {
      const items = grouped.get(category).sort((a, b) => {
        const rank = (STATUS_RANK[a.status] ?? 9) - (STATUS_RANK[b.status] ?? 9);
        return rank !== 0 ? rank : a.name.localeCompare(b.name);
      });

      const section = document.createElement("section");
      section.className = "group";

      const heading = document.createElement("h2");
      heading.textContent = `${categoryLabel(category)}`;
      const count = document.createElement("span");
      count.className = "count";
      count.textContent = String(items.length);
      heading.append(count);

      const grid = document.createElement("div");
      grid.className = "grid";

      for (const service of items) {
        grid.append(buildCard(service));
      }

      section.append(heading, grid);
      els.groups.append(section);
    }

    updateSummary(services.length);
  }

  function updateSummary(shown) {
    const inRegion = (state.snapshot?.services || []).filter((service) =>
      (service.regions || []).includes(state.region));

    const total = state.view === "favorites"
      ? inRegion.filter((service) => state.favorites.has(service.slug)).length
      : inRegion.length;

    els.summary.textContent = t("summary", {
      count: shown,
      total,
      region: regionLabel(state.region),
    });

    els.empty.hidden = shown > 0;
    if (shown === 0) {
      els.empty.textContent = state.view === "favorites" && state.snapshot
        ? t("emptyFavorites")
        : t("empty");
    }
  }

  function render() {
    renderRegions();
    renderGroups();

    els.updated.textContent = formatTime(state.snapshot?.generated_at);
    els.updated.dateTime = state.snapshot?.generated_at || "";

    for (const button of document.querySelectorAll("[data-view]")) {
      button.setAttribute("aria-pressed", String(button.dataset.view === state.view));
    }
  }

  function showError(visible) {
    els.banner.hidden = !visible;
  }

  async function refresh() {
    try {
      const response = await fetch(`${DATA_URL}?t=${Date.now()}`, { cache: "no-store" });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const snapshot = await response.json();
      if (!snapshot || !Array.isArray(snapshot.services)) {
        throw new Error("invalid snapshot");
      }

      state.snapshot = snapshot;
      showError(false);
      render();
    } catch {
      showError(true);
      render();
    }
  }

  function toggleFavorite(slug, name) {
    if (state.favorites.has(slug)) {
      state.favorites.delete(slug);
    } else {
      state.favorites.add(slug);
    }

    writeStore(STORAGE.favorites, JSON.stringify([...state.favorites]));

    const star = document.querySelector(`.star[data-slug="${CSS.escape(slug)}"]`);
    if (star) {
      const favorite = state.favorites.has(slug);
      star.setAttribute("aria-pressed", String(favorite));
      star.setAttribute("aria-label", t(favorite ? "removeFavorite" : "addFavorite", { name }));
      star.textContent = favorite ? "★" : "☆";
    }

    if (state.view === "favorites") {
      renderGroups();
    }
  }

  function setLanguage(lang) {
    state.lang = lang === "pt-br" ? "pt-br" : "en";
    writeStore(STORAGE.lang, state.lang);
    applyStaticTranslations();
    render();
  }

  function bind() {
    els.langToggle.addEventListener("click", () => {
      setLanguage(state.lang === "en" ? "pt-br" : "en");
    });

    els.search.addEventListener("input", (event) => {
      state.query = event.target.value;
      renderGroups();
    });

    els.retry.addEventListener("click", () => {
      refresh();
    });

    els.regions.addEventListener("click", (event) => {
      const button = event.target.closest("button[data-region]");
      if (!button) return;

      state.region = button.dataset.region;
      writeStore(STORAGE.region, state.region);
      render();
    });

    document.querySelector(".segmented").addEventListener("click", (event) => {
      const button = event.target.closest("button[data-view]");
      if (!button) return;

      state.view = button.dataset.view;
      writeStore(STORAGE.view, state.view);
      render();
    });

    els.groups.addEventListener("click", (event) => {
      const button = event.target.closest(".star[data-slug]");
      if (!button) return;

      const card = button.closest(".card");
      const name = card ? card.querySelector("h3").textContent : button.dataset.slug;
      toggleFavorite(button.dataset.slug, name);
    });

    document.addEventListener("visibilitychange", () => {
      if (!document.hidden) {
        refresh();
      }
    });
  }

  function init() {
    state.lang = detectLanguage();
    state.region = readStore(STORAGE.region) || "global";
    state.view = readStore(STORAGE.view) === "favorites" ? "favorites" : "all";
    state.favorites = loadFavorites();

    bind();
    applyStaticTranslations();
    render();
    refresh();

    state.timer = window.setInterval(() => {
      if (!document.hidden) {
        refresh();
      }
    }, REFRESH_MS);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
