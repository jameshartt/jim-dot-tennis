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

// Request permission and subscribe to push notifications
async function subscribeToPushNotifications() {
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
    console.log('Fetching VAPID public key...');
    const response = await fetch('/api/vapid-public-key');
    const data = await response.json();
    console.log('Received VAPID public key response:', data);
    const vapidPublicKey = data.publicKey;
    console.log('VAPID public key:', vapidPublicKey);

    // Convert the VAPID public key to a Uint8Array
    console.log('Converting VAPID key to Uint8Array...');
    const applicationServerKey = urlBase64ToUint8Array(vapidPublicKey);
    console.log('Converted application server key:', applicationServerKey);
    console.log('Application server key length:', applicationServerKey.length);
    console.log('Application server key first few bytes:', Array.from(applicationServerKey.slice(0, 5)));

    // Get the service worker registration
    console.log('Getting service worker registration...');
    const registration = await navigator.serviceWorker.ready;
    console.log('Service worker registration:', registration);
    console.log('Service worker scope:', registration.scope);
    console.log('Service worker state:', registration.active ? registration.active.state : 'no active worker');
    
    // Subscribe to push notifications
    console.log('Attempting to subscribe to push notifications...');
    try {
      console.log('PushManager available:', !!registration.pushManager);
      
      // Check permission state with fallback
      let permissionState = 'prompt';
      try {
        permissionState = await registration.pushManager.permissionState();
        console.log('PushManager permission state:', permissionState);
      } catch (error) {
        console.log('PushManager.permissionState() not supported, using Notification.permission instead');
        permissionState = Notification.permission;
      }
      
      // If permission is denied, we can't proceed
      if (permissionState === 'denied') {
        console.log('Push permission is denied');
        throw new Error('Push permission is denied');
      }

      // Log current subscription state
      const existingSubscription = await registration.pushManager.getSubscription();
      console.log('Existing subscription:', existingSubscription ? 'Yes' : 'No');
      if (existingSubscription) {
        console.log('Unsubscribing from existing subscription...');
        await existingSubscription.unsubscribe();
      }

      // Verify the application server key
      console.log('Application server key details:');
      console.log('- Type:', applicationServerKey.constructor.name);
      console.log('- Length:', applicationServerKey.length);
      console.log('- First few bytes:', Array.from(applicationServerKey.slice(0, 5)));
      console.log('- Is Uint8Array:', applicationServerKey instanceof Uint8Array);
      
      // Attempt subscription with detailed error handling
      console.log('Attempting push subscription with options:', {
        userVisibleOnly: true,
        applicationServerKeyLength: applicationServerKey.length
      });
      
      try {
        const subscription = await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: applicationServerKey
        });
        
        console.log('Push subscription successful:');
        console.log('- Endpoint:', subscription.endpoint);
        console.log('- Keys:', subscription.keys);
        console.log('- Options:', subscription.options);
        
        // Verify the subscription
        const subscriptionJson = subscription.toJSON();
        console.log('Subscription JSON:', subscriptionJson);
        
        if (!subscriptionJson.keys || !subscriptionJson.keys.p256dh || !subscriptionJson.keys.auth) {
          throw new Error('Invalid subscription: missing required keys');
        }
        
        // Send the subscription to the server
        console.log('Sending subscription to server...');
        const serverResponse = await fetch('/api/push/subscribe', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(subscriptionJson)
        });
        
        if (!serverResponse.ok) {
          const errorText = await serverResponse.text();
          throw new Error(`Server responded with ${serverResponse.status}: ${errorText}`);
        }
        
        console.log('Successfully subscribed to push notifications');
        return true;
      } catch (subscribeError) {
        console.error('Push subscription failed:', subscribeError);
        console.error('Error name:', subscribeError.name);
        console.error('Error message:', subscribeError.message);
        console.error('Error stack:', subscribeError.stack);
        
        // Additional diagnostic information
        console.error('Diagnostic information:');
        console.error('- Service Worker state:', registration.active ? registration.active.state : 'no active worker');
        console.error('- Service Worker scope:', registration.scope);
        console.error('- PushManager available:', !!registration.pushManager);
        console.error('- Notification permission:', Notification.permission);
        console.error('- Application server key valid:', applicationServerKey instanceof Uint8Array);
        console.error('- Application server key length:', applicationServerKey.length);
        
        // Try to get more specific error information
        if (subscribeError.name === 'NotAllowedError') {
          console.error('Push subscription not allowed. This might be due to:');
          console.error('- Browser blocking push notifications');
          console.error('- User denying permission');
          console.error('- Invalid VAPID key');
        } else if (subscribeError.name === 'NotSupportedError') {
          console.error('Push subscription not supported. This might be due to:');
          console.error('- Browser not supporting push notifications');
          console.error('- Invalid service worker configuration');
          console.error('- Missing or invalid VAPID key');
        } else if (subscribeError.name === 'AbortError') {
          console.error('Push subscription aborted. This might be due to:');
          console.error('- Service worker not active');
          console.error('- Network issues');
          console.error('- Browser security restrictions');
        }
        
        throw subscribeError;
      }
    } catch (error) {
      console.error('Error in subscribeToPushNotifications:', error);
      console.error('Error name:', error.name);
      console.error('Error message:', error.message);
      console.error('Error stack:', error.stack);
      return false;
    }
  } catch (error) {
    console.error('Error in subscribeToPushNotifications:', error);
    console.error('Error name:', error.name);
    console.error('Error message:', error.message);
    console.error('Error stack:', error.stack);
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
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        endpoint: subscription.endpoint
      })
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
  console.log('checkPushSubscription called');
  if (!isPushNotificationSupported()) {
    console.log('Push notifications not supported in checkPushSubscription');
    return false;
  }

  try {
    console.log('Getting service worker registration...');
    const registration = await navigator.serviceWorker.ready;
    console.log('Service worker registration obtained:', registration);
    
    console.log('Getting push subscription...');
    const subscription = await registration.pushManager.getSubscription();
    console.log('Push subscription result:', subscription);
    
    return !!subscription;
  } catch (error) {
    console.error('Error in checkPushSubscription:', error);
    // Log the full error details
    console.error('Error name:', error.name);
    console.error('Error message:', error.message);
    console.error('Error stack:', error.stack);
    return false;
  }
}

// Send a test notification (for debugging)
async function sendTestNotification() {
  try {
    const response = await fetch('/api/push/test', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        message: 'This is a test notification from Jim.Tennis!'
      })
    });
    
    const data = await response.json();
    console.log('Test notification response:', data);
    return true;
  } catch (error) {
    console.error('Error sending test notification:', error);
    return false;
  }
} 