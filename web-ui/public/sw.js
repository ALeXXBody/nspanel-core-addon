// public/sw.js
// Service worker: cache-first for content-hashed build assets.
// The previous worker was a no-op passthrough (respondWith(fetch(request))),
// which added a worker hop to every request and provided zero caching or
// offline behavior: recovering from Wi-Fi loss re-downloaded the whole app.
// Assets under /webapp/assets/ are content-hashed, so cache-first for them is
// always safe; everything else (e.g. index.html) is handled natively.
const ASSET_CACHE = "nspm-assets-v2";

self.addEventListener("install", (event) => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      // Drop caches from previous versions of this worker.
      const names = await caches.keys();
      await Promise.all(names.filter((n) => n !== ASSET_CACHE).map((n) => caches.delete(n)));
      await clients.claim();
    })(),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || !url.pathname.startsWith("/webapp/assets/")) {
    // Not a build asset: let the browser handle it natively.
    return;
  }
  event.respondWith(
    caches.match(event.request).then(
      (hit) =>
        hit ||
        fetch(event.request).then((response) => {
          if (response.ok) {
            const copy = response.clone();
            caches.open(ASSET_CACHE).then((cache) => cache.put(event.request, copy));
          }
          return response;
        }),
    ),
  );
});
