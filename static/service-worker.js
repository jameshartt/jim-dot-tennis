// Service Worker for Jim.Tennis
console.log('Service Worker: Starting...');

// Install event - cache static assets
self.addEventListener('install', function(event) {
  console.log('Service Worker: Installing...');
  event.waitUntil(
    caches.open('jim-tennis-v1').then(function(cache) {
      console.log('Service Worker: Caching static assets...');
      return cache.addAll([
        '/',
        '/static/icon-192.svg',
        '/static/icon-512.svg',
        '/static/manifest.json'
      ]);
    }).then(function() {
      console.log('Service Worker: Skip waiting...');
      return self.skipWaiting();
    })
  );
});

// Activate event - clean up old caches
self.addEventListener('activate', function(event) {
  console.log('Service Worker: Activating...');
  event.waitUntil(
    caches.keys().then(function(cacheNames) {
      return Promise.all(
        cacheNames.map(function(cacheName) {
          if (cacheName !== 'jim-tennis-v1') {
            console.log('Service Worker: Deleting old cache:', cacheName);
            return caches.delete(cacheName);
          }
        })
      );
    }).then(function() {
      console.log('Service Worker: Claiming clients...');
      return self.clients.claim();
    })
  );
});

// Handle push events - cross-platform compatible
self.addEventListener('push', function(event) {
  console.log('Service Worker: Push event received');

  var title = 'Jim.Tennis';
  var options = {
    body: 'New notification',
    icon: '/static/icon-192.svg',
    badge: '/static/icon-192.svg',
    data: { url: '/' }
  };

  if (event.data) {
    try {
      var payload = event.data.json();
      console.log('Service Worker: Push payload:', payload);

      // Support both structured payloads (title/body) and simple message payloads
      if (payload.title) {
        title = payload.title;
      }

      options.body = payload.body || payload.message || options.body;

      if (payload.icon) {
        options.icon = payload.icon;
      }
      if (payload.badge) {
        options.badge = payload.badge;
      }
      if (payload.data) {
        options.data = payload.data;
      }
      if (payload.tag) {
        options.tag = payload.tag;
      }
    } catch (e) {
      console.log('Service Worker: JSON parse failed, using text:', e);
      options.body = event.data.text() || options.body;
    }
  }

  // Set cross-platform compatible options
  options.requireInteraction = true;
  options.renotify = true;
  options.tag = options.tag || 'jim-tennis-' + Date.now();
  options.vibrate = [100, 50, 100];
  options.actions = [
    { action: 'open', title: 'Open' },
    { action: 'close', title: 'Close' }
  ];

  event.waitUntil(
    // Also notify any focused clients so they can show an in-app toast
    // (iOS PWA suppresses banners when the app is in the foreground)
    clients.matchAll({ type: 'window', includeUncontrolled: true })
      .then(function(clientList) {
        clientList.forEach(function(client) {
          client.postMessage({ type: 'push-received', title: title, body: options.body });
        });
      })
      .then(function() {
        return self.registration.showNotification(title, options);
      })
      .catch(function(error) {
        console.error('Service Worker: Error showing notification:', error);
        // Fallback with minimal options (Safari compatibility)
        return self.registration.showNotification(title, {
          body: options.body,
          icon: options.icon,
          data: options.data
        });
      })
  );
});

// Handle notification clicks
self.addEventListener('notificationclick', function(event) {
  console.log('Service Worker: Notification clicked, action:', event.action);
  event.notification.close();

  if (event.action === 'close') {
    return;
  }

  var url = (event.notification.data && event.notification.data.url) || '/';
  console.log('Service Worker: Opening URL:', url);

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true })
      .then(function(clientList) {
        // Focus existing window if open
        for (var i = 0; i < clientList.length; i++) {
          var client = clientList[i];
          if (client.url.indexOf(url) !== -1 && 'focus' in client) {
            return client.focus();
          }
        }
        // Open new window
        if (clients.openWindow) {
          return clients.openWindow(url);
        }
      })
      .catch(function(error) {
        console.error('Service Worker: Error handling notification click:', error);
      })
  );
});

// Handle push subscription change (browser rotates keys)
self.addEventListener('pushsubscriptionchange', function(event) {
  console.log('Service Worker: Push subscription changed');
  event.waitUntil(
    self.registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: event.oldSubscription ? event.oldSubscription.options.applicationServerKey : null
    }).then(function(subscription) {
      console.log('Service Worker: Re-subscribed:', subscription);
      return fetch('/api/push/subscribe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(subscription)
      });
    })
  );
});
