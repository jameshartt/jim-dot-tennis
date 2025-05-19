// Log when the service worker is installing
self.addEventListener('install', function(event) {
  console.log('Service Worker installing...');
  // Skip waiting to activate immediately
  event.waitUntil(self.skipWaiting());
});

// Log when the service worker is activating
self.addEventListener('activate', function(event) {
  console.log('Service Worker activating...');
  // Claim clients to ensure the service worker is in control
  event.waitUntil(
    Promise.all([
      // Take control of all clients as soon as it activates
      self.clients.claim(),
      // Clean up old caches if needed
      caches.keys().then(function(cacheNames) {
        return Promise.all(
          cacheNames.map(function(cacheName) {
            return caches.delete(cacheName);
          })
        );
      })
    ])
  );
});

// Log when the service worker is fetching
self.addEventListener('fetch', function(event) {
  console.log('Service Worker fetching:', event.request.url);
  event.respondWith(fetch(event.request));
});

// Handle push events
self.addEventListener('push', function(event) {
  console.log('Push event received');
  let payload = {};
  try {
    payload = event.data.json();
    console.log('Push payload:', payload);
  } catch (e) {
    console.log('Push payload error:', e);
    payload = {
      message: event.data ? event.data.text() : 'No payload'
    };
  }

  const title = 'Jim.Tennis';
  const options = {
    body: payload.message || 'New notification',
    icon: '/static/icon-192.svg',
    badge: '/static/icon-192.svg',
    data: {
      dateOfArrival: Date.now(),
      url: self.location.origin
    }
  };

  console.log('Showing notification:', options);
  event.waitUntil(
    self.registration.showNotification(title, options)
  );
});

// Handle notification clicks
self.addEventListener('notificationclick', function(event) {
  console.log('Notification clicked');
  event.notification.close();

  event.waitUntil(
    clients.matchAll({
      type: 'window'
    }).then(function(clientList) {
      const url = event.notification.data.url || '/';
      console.log('Opening URL:', url);
      
      // Check if there is already a window/tab open with the target URL
      for (var i = 0; i < clientList.length; i++) {
        var client = clientList[i];
        if (client.url === url && 'focus' in client) {
          console.log('Focusing existing window');
          return client.focus();
        }
      }
      
      // If no window/tab is open, open a new one
      if (clients.openWindow) {
        console.log('Opening new window');
        return clients.openWindow(url);
      }
    })
  );
});
