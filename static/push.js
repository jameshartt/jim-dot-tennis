// Function to convert base64 string to Uint8Array
function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - base64String.length % 4) % 4);
  const base64 = (base64String + padding)
    .replace(/-/g, '+')
    .replace(/_/g, '/');

  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);

  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

// Check if push notifications are supported
function isPushNotificationSupported() {
  return 'serviceWorker' in navigator && 'PushManager' in window;
}

// Detect if running on iOS
function isIOS() {
  return /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
}

// Detect if app is installed as PWA (standalone mode)
function isStandalone() {
  return window.matchMedia('(display-mode: standalone)').matches ||
    window.navigator.standalone === true;
}

// Request permission and subscribe to push notifications
async function subscribeToPushNotifications(playerToken) {
  if (!isPushNotificationSupported()) {
    console.log('Push notifications not supported');
    return false;
  }

  try {
    // Request permission
    const permission = await Notification.requestPermission();
    console.log('Notification permission status:', permission);
    if (permission !== 'granted') {
      console.log('Notification permission denied');
      return false;
    }

    // Get VAPID public key from the server
    const response = await fetch('/api/vapid-public-key');
    const data = await response.json();
    const vapidPublicKey = data.publicKey;

    // Convert the VAPID public key to a Uint8Array
    const applicationServerKey = urlBase64ToUint8Array(vapidPublicKey);

    // Get the service worker registration
    const registration = await navigator.serviceWorker.ready;

    // Unsubscribe from any existing subscription
    const existingSubscription = await registration.pushManager.getSubscription();
    if (existingSubscription) {
      await existingSubscription.unsubscribe();
    }

    // Subscribe to push notifications
    const subscription = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: applicationServerKey
    });

    const subscriptionJson = subscription.toJSON();

    if (!subscriptionJson.keys || !subscriptionJson.keys.p256dh || !subscriptionJson.keys.auth) {
      throw new Error('Invalid subscription: missing required keys');
    }

    // Include player token in the subscription data
    const payload = Object.assign({}, subscriptionJson);
    if (playerToken) {
      payload.playerToken = playerToken;
    }

    // Send the subscription to the server
    const serverResponse = await fetch('/api/push/subscribe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    if (!serverResponse.ok) {
      const errorText = await serverResponse.text();
      throw new Error('Server responded with ' + serverResponse.status + ': ' + errorText);
    }

    console.log('Successfully subscribed to push notifications');
    return true;
  } catch (error) {
    console.error('Error subscribing to push notifications:', error);
    return false;
  }
}

// Unsubscribe from push notifications
async function unsubscribeFromPushNotifications() {
  if (!isPushNotificationSupported()) {
    return false;
  }

  try {
    const registration = await navigator.serviceWorker.ready;
    const subscription = await registration.pushManager.getSubscription();

    if (!subscription) {
      return false;
    }

    // Unsubscribe on the client
    await subscription.unsubscribe();

    // Notify the server
    await fetch('/api/push/unsubscribe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: subscription.endpoint })
    });

    console.log('Unsubscribed from push notifications');
    return true;
  } catch (error) {
    console.error('Error unsubscribing from push notifications:', error);
    return false;
  }
}

// Check the current subscription status
async function checkPushSubscription() {
  if (!isPushNotificationSupported()) {
    return false;
  }

  try {
    const registration = await navigator.serviceWorker.ready;
    const subscription = await registration.pushManager.getSubscription();
    return !!subscription;
  } catch (error) {
    console.error('Error checking push subscription:', error);
    return false;
  }
}

// Send a test notification (for debugging)
async function sendTestNotification() {
  try {
    const response = await fetch('/api/push/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: 'This is a test notification from Jim.Tennis!' })
    });

    const data = await response.json();
    console.log('Test notification response:', data);
    return true;
  } catch (error) {
    console.error('Error sending test notification:', error);
    return false;
  }
}
