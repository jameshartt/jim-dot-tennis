self.addEventListener('install', function(event) {
  self.skipWaiting();
});

self.addEventListener('activate', function(event) {
  // Clean up old caches if needed
});

self.addEventListener('fetch', function(event) {
  event.respondWith(fetch(event.request));
});
