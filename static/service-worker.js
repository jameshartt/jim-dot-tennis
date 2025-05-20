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

// Handle push events
self.addEventListener('push', function(event) {
  console.log('Service Worker: Push event received');
  
  // Ensure we have permission to show notifications
  if (!self.Notification || self.Notification.permission !== 'granted') {
    console.log('Service Worker: No notification permission');
    return;
  }
  
  let payload = {};
  try {
    payload = event.data.json();
    console.log('Service Worker: Push payload:', payload);
  } catch (e) {
    console.log('Service Worker: Push payload error:', e);
    payload = {
      message: event.data ? event.data.text() : 'No payload'
    };
  }

  // Check if this is a Safari-style payload
  const isSafariPayload = payload.title !== undefined && payload.body !== undefined;
  console.log('Service Worker: Is Safari payload:', isSafariPayload);
  
  // For Safari, we need to ensure all required fields are present
  const title = isSafariPayload ? payload.title : 'Jim.Tennis';
  const options = {
    body: isSafariPayload ? payload.body : (payload.message || 'New notification'),
    icon: payload.icon || '/static/icon-192.svg',
    badge: payload.badge || '/static/icon-192.svg',
    data: payload.data || {
      dateOfArrival: Date.now(),
      url: self.location.origin
    },
    requireInteraction: true,
    tag: payload.tag || 'default',
    renotify: payload.renotify !== undefined ? payload.renotify : true,
    timestamp: payload.data?.dateOfArrival || Date.now(),
    // Safari requires these specific options
    silent: false,
    vibrate: [100, 50, 100],
    actions: payload.actions || [
      {
        action: 'open',
        title: 'Open'
      },
      {
        action: 'close',
        title: 'Close'
      }
    ]
  };

  // Log the full notification options for debugging
  console.log('Service Worker: Notification options:', JSON.stringify(options, null, 2));

  // Show the notification
  event.waitUntil(
    self.registration.showNotification(title, options)
      .then(() => {
        console.log('Service Worker: Notification shown successfully');
      })
      .catch(error => {
        console.error('Service Worker: Error showing notification:', error);
        // Try to show a fallback notification if the main one fails
        return self.registration.showNotification('Jim.Tennis', {
          body: 'New notification',
          icon: '/static/icon-192.svg',
          badge: '/static/icon-192.svg'
        });
      })
  );
});

// Handle notification clicks with improved Safari support
self.addEventListener('notificationclick', function(event) {
  console.log('Service Worker: Notification clicked, action:', event.action);
  event.notification.close();

  if (event.action === 'close') {
    return;
  }

  // Get the URL from the notification data
  const url = event.notification.data?.url || '/';
  console.log('Service Worker: Opening URL:', url);

  event.waitUntil(
    // First, try to focus an existing window
    clients.matchAll({
      type: 'window',
      includeUncontrolled: true
    }).then(function(clientList) {
      // Check if there is already a window/tab open with the target URL
      for (const client of clientList) {
        if (client.url === url && 'focus' in client) {
          console.log('Service Worker: Focusing existing window');
          return client.focus();
        }
      }

      // If no window/tab is open, try to open a new one
      if (clients.openWindow) {
        console.log('Service Worker: Opening new window');
        return clients.openWindow(url).catch(function(error) {
          console.error('Service Worker: Error opening window:', error);
          // If opening a new window fails, try to open in the current window
          return clients.openWindow('/');
        });
      }
    }).catch(function(error) {
      console.error('Service Worker: Error handling notification click:', error);
    })
  );
});

// Handle push subscription change
self.addEventListener('pushsubscriptionchange', function(event) {
  console.log('Service Worker: Push subscription changed');
  event.waitUntil(
    self.registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: event.oldSubscription ? event.oldSubscription.options.applicationServerKey : null
    }).then(function(subscription) {
      console.log('Service Worker: New subscription:', subscription);
      // Send the new subscription to the server
      return fetch('/api/push/subscribe', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(subscription)
      });
    })
  );
});
