(() => {
  "use strict";

  const I18N = (window.SERAPH_I18N = window.SERAPH_I18N || { en: {} });
  const DATA_URL = "data/state.json";
  const REFRESH_MS = 60_000;
  const STORAGE = {
    lang: "seraph.lang.v1",
    region: "seraph.region.v1",
    favorites: "seraph.favorites.v1",
    collections: "seraph.collections.v1",
    activeCollection: "seraph.collection.v1",
    view: "seraph.view.v1",
    collapsed: "seraph.collapsed.v1",
  };
  const CATEGORY_ORDER = [
    "banking",
    "payments",
    "telecom",
    "messaging",
    "social",
    "ai",
    "cloud",
    "streaming",
    "gaming",
    "transport",
    "ecommerce",
    "gov",
  ];
  const STATUS_RANK = { major: 0, degraded: 1, unknown: 2, operational: 3 };
  const AVAILABILITY_WINDOW_MS = 24 * 60 * 60 * 1000;
  const MIN_AVAILABILITY_POINTS = 3;
  const STALE_AFTER_MS = 20 * 60 * 1000;
  const SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
  const prefersReducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;

  const state = {
    lang: "en",
    region: "global",
    view: "all",
    query: "",
    favorites: new Set(),
    collections: [],
    activeCollection: "",
    refreshing: false,
    collapsed: new Set(),
    statusFilter: "",
    visibleCategories: [],
    snapshot: null,
    timer: null,
  };

  const els = {
    langToggle: document.getElementById("lang-toggle"),
    refresh: document.getElementById("refresh"),
    meta: document.getElementById("update-meta"),
    search: document.getElementById("search"),
    regions: document.getElementById("regions"),
    segmented: document.querySelector(".segmented"),
    summary: document.getElementById("summary"),
    overview: document.getElementById("overview"),
    availability: document.getElementById("availability"),
    availabilityList: document.getElementById("availability-list"),
    updated: document.getElementById("updated"),
    banner: document.getElementById("banner"),
    bannerText: document.getElementById("banner-text"),
    retry: document.getElementById("retry"),
    groups: document.getElementById("groups"),
    empty: document.getElementById("empty"),
    backToTop: document.getElementById("back-to-top"),
    favoritesTools: document.getElementById("favorites-tools"),
    favoritesStatus: document.getElementById("favorites-status"),
    favoritesExport: document.getElementById("favorites-export"),
    favoritesImport: document.getElementById("favorites-import"),
    favoritesLink: document.getElementById("favorites-link"),
    favoritesFile: document.getElementById("favorites-file"),
    collectionSelect: document.getElementById("collection-select"),
    collectionNew: document.getElementById("collection-new"),
    collectionRename: document.getElementById("collection-rename"),
    collectionDelete: document.getElementById("collection-delete"),
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

  function collectionServices(items) {
    const slugs = new Set();

    for (const item of Array.isArray(items) ? items : []) {
      if (typeof item === "string" && SLUG_PATTERN.test(item)) {
        slugs.add(item);
      }
    }

    return [...slugs];
  }

  function normalizeCollection(entry, index) {
    if (!entry || typeof entry !== "object") {
      return null;
    }

    const id = typeof entry.id === "string" && entry.id.trim()
      ? entry.id.trim()
      : `collection-${index + 1}`;
    const name = typeof entry.name === "string" && entry.name.trim()
      ? entry.name.trim()
      : t("collectionDefaultName");

    return {
      id,
      name,
      services: collectionServices(entry.services),
    };
  }

  function loadCollections() {
    try {
      const raw = readStore(STORAGE.collections);

      if (raw) {
        const parsed = JSON.parse(raw);

        if (Array.isArray(parsed)) {
          const collections = parsed.map(normalizeCollection).filter(Boolean);

          if (collections.length > 0) {
            return collections;
          }
        }
      }
    } catch {
      // Fall through to the legacy favorites migration.
    }

    let legacy = [];

    try {
      const parsed = JSON.parse(readStore(STORAGE.favorites) || "[]");
      legacy = collectionServices(parsed);
    } catch {
      // Start with an empty default collection.
    }

    return [{ id: "default", name: t("favorites"), services: legacy }];
  }

  function activeCollection() {
    return state.collections.find((collection) => collection.id === state.activeCollection)
      || state.collections[0]
      || null;
  }

  function saveCollections() {
    writeStore(STORAGE.collections, JSON.stringify(state.collections));
  }

  function loadCollapsed() {
    try {
      const parsed = JSON.parse(readStore(STORAGE.collapsed) || "[]");
      return new Set(Array.isArray(parsed) ? parsed.filter((item) => typeof item === "string") : []);
    } catch {
      return new Set();
    }
  }

  function saveCollapsed() {
    writeStore(STORAGE.collapsed, JSON.stringify([...state.collapsed]));
  }

  function saveFavorites() {
    const active = activeCollection();

    if (active) {
      active.services = [...state.favorites].sort();
    }

    saveCollections();
    writeStore(STORAGE.favorites, JSON.stringify([...state.favorites]));
  }

  function favoriteSlugs(payload) {
    let items = [];

    if (Array.isArray(payload)) {
      items = payload;
    } else if (payload && Array.isArray(payload.services)) {
      items = payload.services;
    } else if (payload && Array.isArray(payload.favorites)) {
      items = payload.favorites;
    }

    const slugs = new Set();

    for (const item of items) {
      const slug = typeof item === "string" ? item : item?.slug;

      if (typeof slug === "string" && SLUG_PATTERN.test(slug)) {
        slugs.add(slug);
      }
    }

    return [...slugs];
  }

  function setFavoritesStatus(key, vars) {
    els.favoritesStatus.textContent = key ? t(key, vars) : "";
  }

  function renderFavoritesTools() {
    const active = state.view === "favorites";
    els.favoritesTools.hidden = !active;

    if (!active) {
      return;
    }

    const empty = state.favorites.size === 0;
    els.favoritesExport.disabled = empty;
    els.favoritesLink.disabled = empty;

    els.collectionSelect.replaceChildren(...state.collections.map((collection) => {
      const option = document.createElement("option");
      option.value = collection.id;
      option.textContent = collection.name;

      return option;
    }));
    els.collectionSelect.value = state.activeCollection;
    els.collectionDelete.disabled = state.collections.length < 2;
  }

  function setActiveCollection(id) {
    const collection = state.collections.find((item) => item.id === id);

    if (!collection) {
      return;
    }

    state.activeCollection = collection.id;
    state.favorites = new Set(collection.services);
    writeStore(STORAGE.activeCollection, state.activeCollection);
    render();
  }

  function createCollection() {
    const name = (window.prompt(t("collectionPromptNew"), t("collectionDefaultName")) || "").trim();

    if (!name) {
      return;
    }

    const id = crypto.randomUUID?.() || `collection-${Date.now()}`;

    state.collections.push({ id, name, services: [] });
    state.activeCollection = id;
    state.favorites = new Set();
    writeStore(STORAGE.activeCollection, state.activeCollection);
    saveFavorites();
    render();
    setFavoritesStatus("collectionCreated", { name });
  }

  function renameCollection() {
    const collection = activeCollection();

    if (!collection) {
      return;
    }

    const name = (window.prompt(t("collectionPromptRename"), collection.name) || "").trim();

    if (!name || name === collection.name) {
      return;
    }

    collection.name = name;
    saveCollections();
    render();
    setFavoritesStatus("collectionRenamed", { name });
  }

  function deleteCollection() {
    const collection = activeCollection();

    if (!collection) {
      return;
    }

    if (state.collections.length < 2) {
      setFavoritesStatus("collectionLast");

      return;
    }

    if (!window.confirm(t("collectionConfirmDelete", { name: collection.name }))) {
      return;
    }

    const name = collection.name;
    state.collections = state.collections.filter((item) => item.id !== collection.id);
    state.activeCollection = state.collections[0].id;
    state.favorites = new Set(state.collections[0].services);
    writeStore(STORAGE.activeCollection, state.activeCollection);
    saveFavorites();
    render();
    setFavoritesStatus("collectionDeleted", { name });
  }

  function exportFavorites() {
    if (state.favorites.size === 0) {
      setFavoritesStatus("favoritesNoFavorites");

      return;
    }

    const names = new Map((state.snapshot?.services || []).map((service) => [service.slug, service.name]));
    const services = [...state.favorites].sort().map((slug) => ({
      slug,
      name: names.get(slug) || slug,
    }));
    const payload = {
      version: 1,
      exported_at: new Date().toISOString(),
      collection: activeCollection()?.name || "",
      services,
    };
    const blob = new Blob([`${JSON.stringify(payload, null, 2)}\n`], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");

    link.href = url;
    link.download = `seraph-favorites-${new Date().toISOString().slice(0, 10)}.json`;
    document.body.append(link);
    link.click();
    link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 1_000);
    setFavoritesStatus("favoritesExported", { count: services.length });
  }

  async function importFavoritesFile(file) {
    try {
      const payload = JSON.parse(await file.text());
      const slugs = favoriteSlugs(payload);

      if (slugs.length === 0) {
        setFavoritesStatus("favoritesImportEmpty");

        return;
      }

      const before = state.favorites.size;
      for (const slug of slugs) {
        state.favorites.add(slug);
      }

      saveFavorites();
      render();

      const added = state.favorites.size - before;
      setFavoritesStatus(
        added === 0 ? "favoritesImportedNone" : "favoritesImported",
        { count: added },
      );
    } catch {
      setFavoritesStatus("favoritesImportInvalid");
    }
  }

  async function copyFavoritesLink() {
    if (state.favorites.size === 0) {
      setFavoritesStatus("favoritesNoFavorites");

      return;
    }

    const url = new URL(window.location.href);
    url.hash = "";
    url.searchParams.set("services", [...state.favorites].sort().join(","));

    try {
      if (!navigator.clipboard?.writeText) {
        throw new Error("clipboard unavailable");
      }

      await navigator.clipboard.writeText(url.toString());
      setFavoritesStatus("favoritesLinkCopied");
    } catch {
      window.prompt(t("favoritesLinkPrompt"), url.toString());
      setFavoritesStatus("favoritesLinkFailed");
    }
  }

  function applySharedFavorites() {
    const url = new URL(window.location.href);
    const raw = url.searchParams.get("services");

    if (!raw) {
      return;
    }

    const slugs = raw.split(",").map((slug) => slug.trim()).filter((slug) => SLUG_PATTERN.test(slug));
    url.searchParams.delete("services");
    window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);

    if (slugs.length === 0) {
      return;
    }

    for (const slug of slugs) {
      state.favorites.add(slug);
    }

    saveFavorites();
    state.view = "favorites";
    writeStore(STORAGE.view, state.view);
    setFavoritesStatus("favoritesImportedLink", { count: slugs.length });
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

    els.langToggle.setAttribute("aria-checked", String(state.lang === "pt-br"));
    els.langToggle.setAttribute("aria-label", t("language"));
    els.langToggle.dataset.lang = state.lang;
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

    els.regions.hidden = regions.length < 2;

    if (els.regions.hidden) {
      return;
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

  function scopedServices() {
    return (state.snapshot?.services || []).filter((service) => {
      if (!(service.regions || []).includes(state.region)) {
        return false;
      }

      return state.view !== "favorites" || state.favorites.has(service.slug);
    });
  }

  function matchesStatus(service, filter) {
    if (filter === "operational") {
      return service.status === "operational";
    }

    if (filter === "attention") {
      return service.status === "degraded" || service.status === "unknown";
    }

    if (filter === "outage") {
      return service.status === "major";
    }

    return true;
  }

  function matchesFilters(service) {
    if (!(service.regions || []).includes(state.region)) {
      return false;
    }

    if (state.view === "favorites" && !state.favorites.has(service.slug)) {
      return false;
    }

    if (!matchesStatus(service, state.statusFilter)) {
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

  function formatRelative(value) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return "";
    }

    const diff = Math.max(0, Date.now() - date.getTime());
    if (diff < 60_000) {
      return t("justNow");
    }

    try {
      const formatter = new Intl.RelativeTimeFormat(
        state.lang === "pt-br" ? "pt-BR" : "en",
        { numeric: "auto" },
      );
      const minutes = Math.floor(diff / 60_000);

      if (minutes < 60) {
        return formatter.format(-minutes, "minute");
      }

      const hours = Math.floor(minutes / 60);
      if (hours < 24) {
        return formatter.format(-hours, "hour");
      }

      return formatter.format(-Math.floor(hours / 24), "day");
    } catch {
      return "";
    }
  }

  function buildCard(service) {
    const article = document.createElement("article");
    article.className = "card";
    article.dataset.status = service.status || "unknown";

    const targetURL = service.status_url || service.official_url;

    if (targetURL) {
      article.classList.add("card--linked");

      const link = document.createElement("a");
      link.className = "card-link";
      link.href = targetURL;
      link.target = "_blank";
      link.rel = "noopener noreferrer";
      link.setAttribute(
        "aria-label",
        t(service.status_url ? "openStatus" : "openOfficial", { name: service.name }),
      );
      article.append(link);
    }

    const header = document.createElement("header");

    const title = document.createElement("h3");
    title.textContent = service.name;

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

    const tags = document.createElement("div");
    tags.className = "tags";

    const categoryTag = document.createElement("span");
    categoryTag.className = "tag";
    categoryTag.textContent = categoryLabel(service.category);
    tags.append(categoryTag);

    for (const region of service.regions || []) {
      const regionTag = document.createElement("span");
      regionTag.className = "tag tag--region";
      regionTag.textContent = regionLabel(region);
      tags.append(regionTag);
    }

    article.append(header, tags, status, details);

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

    state.visibleCategories = order;

    for (const category of order) {
      const items = grouped.get(category).sort((a, b) => {
        const rank = (STATUS_RANK[a.status] ?? 9) - (STATUS_RANK[b.status] ?? 9);
        return rank !== 0 ? rank : a.name.localeCompare(b.name);
      });
      const attentionCount = items.filter((service) => service.status !== "operational").length;
      const attentionByStatus = { major: 0, degraded: 0, unknown: 0 };

      for (const service of items) {
        if (service.status in attentionByStatus) {
          attentionByStatus[service.status]++;
        }
      }

      const label = categoryLabel(category);
      const collapsed = state.collapsed.has(category);
      const bodyId = `group-body-${category}`;

      const section = document.createElement("section");
      section.className = "group";
      section.dataset.category = category;

      if (collapsed) {
        section.classList.add("collapsed");
      }

      const head = document.createElement("button");
      head.type = "button";
      head.className = "group-head";
      head.setAttribute("aria-controls", bodyId);
      head.setAttribute("aria-expanded", String(!collapsed));
      head.setAttribute("aria-label", categoryAriaLabel(label, items.length, collapsed, attentionCount));

      const title = document.createElement("span");
      title.className = "group-title";
      title.textContent = label;

      const count = document.createElement("span");
      count.className = "count";
      count.textContent = String(items.length);

      head.append(title, count);

      for (const status of ["major", "degraded", "unknown"]) {
        const count = attentionByStatus[status];

        if (count === 0) {
          continue;
        }

        const alert = document.createElement("span");
        alert.className = `group-alert group-alert--${status}`;
        alert.textContent = String(count);
        alert.setAttribute("aria-label", t("attentionStatusCount", {
          count,
          status: statusLabel(status),
        }));
        head.append(alert);
      }

      const toggle = document.createElement("span");
      toggle.className = "group-toggle";
      toggle.setAttribute("aria-hidden", "true");
      toggle.innerHTML = '<span class="chevron"></span>';
      head.append(toggle);

      const body = document.createElement("div");
      body.className = "group-body";
      body.id = bodyId;
      body.setAttribute("aria-hidden", String(collapsed));

      const inner = document.createElement("div");
      inner.className = "group-body-inner";

      const grid = document.createElement("div");
      grid.className = "grid";

      for (const service of items) {
        grid.append(buildCard(service));
      }

      inner.append(grid);
      body.append(inner);

      head.addEventListener("click", () => {
        const isCollapsed = !section.classList.contains("collapsed");
        setGroupCollapsed(section, isCollapsed);

        if (isCollapsed) {
          state.collapsed.add(category);
        } else {
          state.collapsed.delete(category);
        }

        saveCollapsed();
        renderOverview();
      });

      section.append(head, body);
      els.groups.append(section);
    }

    updateSummary(services.length);
    renderOverview();
  }

  function setGroupCollapsed(section, collapsed) {
    const head = section.querySelector(".group-head");
    const body = section.querySelector(".group-body");
    const title = section.querySelector(".group-title");
    const count = section.querySelector(".count");
    const label = title?.textContent || section.dataset.category || "";
    const total = Number(count?.textContent || 0);
    const attention = [...section.querySelectorAll(".group-alert")]
      .reduce((sum, alert) => sum + Number(alert.textContent || 0), 0);

    section.classList.toggle("collapsed", collapsed);
    head?.setAttribute("aria-expanded", String(!collapsed));
    head?.setAttribute("aria-label", categoryAriaLabel(label, total, collapsed, attention));
    body?.setAttribute("aria-hidden", String(collapsed));
  }

  function categoryAriaLabel(label, count, collapsed, attention) {
    let text = t(collapsed ? "expandCategory" : "collapseCategory", { name: label, count });

    if (attention > 0) {
      text += ` · ${t("attentionCount", { count: attention })}`;
    }

    return text;
  }

  function overviewCard(label, value, meta, tone, onClick, pressed) {
    const card = document.createElement(onClick ? "button" : "article");
    card.className = `overview-card${tone ? ` overview-card--${tone}` : ""}`;

    if (onClick) {
      card.type = "button";
      card.classList.add("overview-card--action");
      card.setAttribute("aria-label", `${label}: ${value} · ${meta}`);
      card.addEventListener("click", onClick);

      if (typeof pressed === "boolean") {
        card.setAttribute("aria-pressed", String(pressed));

        if (pressed) {
          card.classList.add("overview-card--active");
        }
      }
    }

    const labelEl = document.createElement("span");
    labelEl.className = "overview-label";
    labelEl.textContent = label;

    const valueEl = document.createElement("strong");
    valueEl.className = "overview-value";
    valueEl.textContent = value;

    const metaEl = document.createElement("span");
    metaEl.className = "overview-meta";
    metaEl.textContent = meta;

    card.append(labelEl, valueEl, metaEl);

    return card;
  }

  function renderOverview() {
    const services = scopedServices();
    const total = services.length;
    const operational = services.filter((service) => service.status === "operational").length;
    const attention = services.filter(
      (service) => service.status === "degraded" || service.status === "unknown",
    ).length;
    const outage = services.filter((service) => service.status === "major").length;
    const categories = new Set(services.map((service) => service.category || "other")).size;
    const percent = total > 0 ? Math.round((operational / total) * 100) : 0;
    const filtering = state.statusFilter !== "";
    const attentionFiltering = state.statusFilter === "attention";
    const operationalFiltering = state.statusFilter === "operational";
    const outageFiltering = state.statusFilter === "outage";

    const allCollapsed = state.visibleCategories.length > 0
      && state.visibleCategories.every((category) => state.collapsed.has(category));
    const actionLabel = state.visibleCategories.length > 0
      ? t(allCollapsed ? "overviewExpandAll" : "overviewCollapseAll")
      : t("overviewNoGroups");

    const attentionMeta = total === 0
      ? t("overviewNoServices")
      : attentionFiltering
        ? t("overviewClearFilter")
        : attention === 0
          ? t("overviewAllGood")
          : t("overviewFilter");
    const operationalMeta = total === 0
      ? t("overviewNoServices")
      : operationalFiltering
        ? t("overviewClearFilter")
        : operational === 0
          ? t("overviewNoneOperational")
          : t("overviewFilter");
    const outageMeta = total === 0
      ? t("overviewNoServices")
      : outageFiltering
        ? t("overviewClearFilter")
        : outage === 0
          ? t("overviewNoOutages")
          : t("overviewFilter");

    els.overview.replaceChildren(
      overviewCard(
        t("overviewServices"),
        String(total),
        filtering ? t("overviewShowAll") : regionLabel(state.region),
        "",
        filtering ? clearStatusFilter : null,
      ),
      overviewCard(
        t("overviewOperational"),
        String(operational),
        operationalMeta,
        "ok",
        operational > 0 || operationalFiltering ? toggleOperationalFilter : null,
        operationalFiltering,
      ),
      overviewCard(
        t("overviewAttention"),
        String(attention),
        attentionMeta,
        attention > 0 ? "attention" : "ok",
        attention > 0 || attentionFiltering ? toggleAttentionFilter : null,
        attentionFiltering,
      ),
      overviewCard(
        t("overviewOutage"),
        String(outage),
        outageMeta,
        outage > 0 ? "outage" : "ok",
        outage > 0 || outageFiltering ? toggleOutageFilter : null,
        outageFiltering,
      ),
      overviewCard(
        t("overviewCategories"),
        String(categories),
        actionLabel,
        state.visibleCategories.length > 0 ? "action" : "",
        state.visibleCategories.length > 0 ? toggleAllCategories : null,
      ),
    );
  }

  function toggleAttentionFilter() {
    toggleStatusFilter("attention");
  }

  function toggleOperationalFilter() {
    toggleStatusFilter("operational");
  }

  function toggleOutageFilter() {
    toggleStatusFilter("outage");
  }

  function toggleStatusFilter(filter) {
    state.statusFilter = state.statusFilter === filter ? "" : filter;

    if (state.statusFilter) {
      const categories = new Set(
        scopedServices()
          .filter((service) => matchesStatus(service, state.statusFilter))
          .map((service) => service.category || "other"),
      );

      for (const category of categories) {
        state.collapsed.delete(category);
      }

      saveCollapsed();
    }

    renderWithTransition();
  }

  function clearStatusFilter() {
    if (!state.statusFilter) {
      return;
    }

    state.statusFilter = "";
    renderWithTransition();
  }

  function renderWithTransition() {
    if (prefersReducedMotion || typeof document.startViewTransition !== "function") {
      render();
      return;
    }

    document.startViewTransition(render);
  }

  function renderAvailability() {
    const generated = Date.parse(state.snapshot?.generated_at || "");
    const now = Number.isNaN(generated) ? Date.now() : generated;
    const cutoff = now - AVAILABILITY_WINDOW_MS;

    const ranked = scopedServices()
      .map((service) => {
        const points = (service.points || []).filter((point) => {
          const at = Date.parse(point.at);
          return !Number.isNaN(at) && at >= cutoff && at <= now + 60_000;
        });

        if (points.length < MIN_AVAILABILITY_POINTS) {
          return null;
        }

        const operational = points.filter((point) => point.status === "operational").length;

        return {
          service,
          points: points.length,
          pointsData: points,
          availability: (operational / points.length) * 100,
        };
      })
      .filter(Boolean)
      .sort((left, right) => (
        right.availability - left.availability
        || right.points - left.points
        || left.service.name.localeCompare(right.service.name)
      ))
      .slice(0, 3);

    if (ranked.length === 0) {
      els.availability.hidden = true;
      els.availabilityList.replaceChildren();

      return;
    }

    els.availability.hidden = false;
    els.availabilityList.replaceChildren(...ranked.map((item, index) => availabilityRow(item, index)));
  }

  function availabilityRow(item, index) {
    const row = document.createElement("li");
    row.className = "availability-item";

    const rank = document.createElement("span");
    rank.className = "availability-rank";
    rank.textContent = String(index + 1);

    const info = document.createElement("span");
    info.className = "availability-info";

    const name = document.createElement("strong");
    name.className = "availability-name";
    name.textContent = item.service.name;

    const meta = document.createElement("span");
    meta.className = "availability-meta";
    meta.textContent = t("availabilitySamples", { count: item.points });

    info.append(name, meta);

    const history = document.createElement("div");
    history.className = "history availability-history";
    history.setAttribute("aria-hidden", "true");

    for (const point of item.pointsData.slice(-30)) {
      const cell = document.createElement("span");
      cell.dataset.status = point.status || "unknown";
      history.append(cell);
    }

    const availability = Math.max(0, Math.min(100, item.availability));

    const value = document.createElement("strong");
    value.className = "availability-value";
    value.textContent = availability >= 100
      ? "100%"
      : `${availability.toFixed(1)}%`;

    row.append(rank, info, history, value);

    return row;
  }

  function toggleAllCategories() {
    if (state.visibleCategories.length === 0) {
      return;
    }

    const allCollapsed = state.visibleCategories.every(
      (category) => state.collapsed.has(category),
    );
    const nextCollapsed = !allCollapsed;
    const sections = new Map(
      [...els.groups.querySelectorAll(".group")].map((section) => [section.dataset.category, section]),
    );

    for (const category of state.visibleCategories) {
      if (nextCollapsed) {
        state.collapsed.add(category);
      } else {
        state.collapsed.delete(category);
      }

      const section = sections.get(category);
      if (section) {
        setGroupCollapsed(section, nextCollapsed);
      }
    }

    saveCollapsed();
    renderOverview();
  }

  function updateSummary(shown) {
    const inRegion = scopedServices();

    const total = state.view === "favorites"
      ? inRegion.filter((service) => state.favorites.has(service.slug)).length
      : inRegion.length;

    const regions = availableRegions();
    const key = regions.length > 1 ? "summary" : "summarySingle";
    const vars = { count: shown, total };

    if (key === "summary") {
      vars.region = regionLabel(state.region);
    }

    els.summary.textContent = t(key, vars);

    els.empty.hidden = shown > 0;
    if (shown === 0) {
      if (state.statusFilter === "attention") {
        els.empty.textContent = t("emptyAttention");
      } else if (state.statusFilter === "operational") {
        els.empty.textContent = t("emptyOperational");
      } else if (state.statusFilter === "outage") {
        els.empty.textContent = t("emptyOutage");
      } else {
        els.empty.textContent = state.view === "favorites" && state.snapshot
          ? t("emptyFavorites")
          : t("empty");
      }
    }
  }

  function render() {
    renderRegions();
    renderGroups();
    renderAvailability();
    renderFavoritesTools();

    const generated = state.snapshot?.generated_at;
    const absolute = formatTime(generated);
    const relative = generated ? formatRelative(generated) : "";
    const parsed = generated ? Date.parse(generated) : Number.NaN;
    const stale = !Number.isNaN(parsed) && Date.now() - parsed > STALE_AFTER_MS;

    els.updated.textContent = relative
      ? `${absolute} · ${relative}${stale ? ` · ${t("stale")}` : ""}`
      : absolute;
    els.updated.dateTime = generated || "";
    els.updated.title = absolute;
    els.meta.classList.toggle("is-stale", stale);
    els.segmented.dataset.view = state.view;

    for (const button of els.segmented.querySelectorAll("button[data-view]")) {
      button.setAttribute("aria-pressed", String(button.dataset.view === state.view));
    }
  }

  function showError(visible) {
    els.banner.hidden = !visible;
  }

  async function refresh() {
    if (state.refreshing) {
      return;
    }

    state.refreshing = true;
    els.refresh.disabled = true;
    els.refresh.setAttribute("aria-busy", "true");

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
    } finally {
      state.refreshing = false;
      els.refresh.disabled = false;
      els.refresh.removeAttribute("aria-busy");
    }
  }

  function toggleFavorite(slug, name) {
    if (state.favorites.has(slug)) {
      state.favorites.delete(slug);
    } else {
      state.favorites.add(slug);
    }

    saveFavorites();

    const star = document.querySelector(`.star[data-slug="${CSS.escape(slug)}"]`);
    if (star) {
      const favorite = state.favorites.has(slug);
      star.setAttribute("aria-pressed", String(favorite));
      star.setAttribute("aria-label", t(favorite ? "removeFavorite" : "addFavorite", { name }));
      star.textContent = favorite ? "★" : "☆";
    }

    renderFavoritesTools();

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

    els.refresh.addEventListener("click", () => {
      refresh();
    });

    els.favoritesExport.addEventListener("click", exportFavorites);
    els.favoritesImport.addEventListener("click", () => {
      els.favoritesFile.click();
    });
    els.favoritesFile.addEventListener("change", async (event) => {
      const file = event.target.files?.[0];
      event.target.value = "";

      if (file) {
        await importFavoritesFile(file);
      }
    });
    els.favoritesLink.addEventListener("click", copyFavoritesLink);
    els.collectionSelect.addEventListener("change", (event) => {
      setActiveCollection(event.target.value);
    });
    els.collectionNew.addEventListener("click", createCollection);
    els.collectionRename.addEventListener("click", renameCollection);
    els.collectionDelete.addEventListener("click", deleteCollection);

    const updateBackToTop = () => {
      const visible = window.scrollY > 480;
      els.backToTop.classList.toggle("show", visible);
      els.backToTop.tabIndex = visible ? 0 : -1;
    };

    window.addEventListener("scroll", updateBackToTop, { passive: true });
    els.backToTop.addEventListener("click", () => {
      window.scrollTo({ top: 0, behavior: prefersReducedMotion ? "auto" : "smooth" });
    });
    updateBackToTop();

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
    state.collections = loadCollections();
    state.activeCollection = readStore(STORAGE.activeCollection) || state.collections[0].id;
    if (!state.collections.some((collection) => collection.id === state.activeCollection)) {
      state.activeCollection = state.collections[0].id;
    }
    state.favorites = new Set(activeCollection().services);
    state.collapsed = loadCollapsed();
    saveFavorites();

    bind();
    applyStaticTranslations();
    applySharedFavorites();
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

  if ("serviceWorker" in navigator) {
    const hadController = Boolean(navigator.serviceWorker.controller);
    let refreshing = false;

    navigator.serviceWorker.addEventListener("controllerchange", () => {
      if (!hadController || refreshing) {
        return;
      }

      refreshing = true;
      window.location.reload();
    });

    window.addEventListener("load", () => {
      navigator.serviceWorker.register("sw.js").catch(() => {
        // PWA support is optional; the dashboard still works without it.
      });
    });
  }
})();
