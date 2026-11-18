const CACHE = "seraph-shell-v5";
const SCOPE = new URL("./", self.location.href);
const SHELL = [
  "./",
  "index.html",
  "style.css",
  "app.js",
  "backdrop.js",
  "i18n/en.js",
  "i18n/pt-br.js",
  "favicon.svg",
  "icon-192.png",
  "icon-512.png",
  "manifest.webmanifest",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE)
      .then((cache) => cache.addAll(SHELL.map((asset) => new Request(asset, { cache: "reload" }))))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(
        keys.filter((key) => key.startsWith("seraph-shell-") && key !== CACHE)
          .map((key) => caches.delete(key)),
      ))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;

  if (request.method !== "GET") {
    return;
  }

  const url = new URL(request.url);

  if (url.origin !== self.location.origin) {
    return;
  }

  const isData = url.pathname.endsWith("/data/state.json");
  const isNavigation = request.mode === "navigate";

  event.respondWith(networkFirst(request, isData || isNavigation));
});

async function networkFirst(request, ignoreSearch) {
  try {
    const response = await fetch(request, { cache: "no-cache" });

    if (response.ok) {
      const cache = await caches.open(CACHE);
      await cache.put(request, response.clone());
    }

    return response;
  } catch {
    const cached = await caches.match(request, { ignoreSearch });
    if (cached) {
      return cached;
    }

    if (request.mode === "navigate") {
      const shell = await caches.match(SCOPE.href);
      if (shell) {
        return shell;
      }
    }

    return Response.error();
  }
}
