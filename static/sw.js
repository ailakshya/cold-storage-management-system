// Service Worker for Cold Storage PWA
const CACHE_NAME = 'cold-storage-v1';

// Install event
self.addEventListener('install', function(event) {
    self.skipWaiting();
});

// Activate event
self.addEventListener('activate', function(event) {
    event.waitUntil(clients.claim());
});

// Fetch event - network first, then cache
self.addEventListener('fetch', function(event) {
    event.respondWith(
        fetch(event.request).catch(function() {
            return caches.match(event.request);
        })
    );
});
