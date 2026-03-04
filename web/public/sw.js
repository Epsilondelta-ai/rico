// Rico Service Worker - Push Notifications

self.addEventListener('push', function(event) {
  console.log('Push 이벤트 수신:', event);

  let data = { title: 'Rico', body: '응답이 도착했습니다' };

  if (event.data) {
    try {
      data = event.data.json();
    } catch (e) {
      data.body = event.data.text();
    }
  }

  const options = {
    body: data.body,
    icon: '/icon.svg',
    badge: '/icon.svg',
    vibrate: [200, 100, 200],
    tag: 'rico-response',
    renotify: true,
    data: {
      url: data.url || '/'
    }
  };

  event.waitUntil(
    self.registration.showNotification(data.title || 'Rico', options)
  );
});

self.addEventListener('notificationclick', function(event) {
  console.log('알림 클릭:', event);
  event.notification.close();

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true })
      .then(function(clientList) {
        // 이미 열린 창이 있으면 포커스
        for (let client of clientList) {
          if (client.url.includes(self.location.origin) && 'focus' in client) {
            return client.focus();
          }
        }
        // 없으면 새 창 열기
        if (clients.openWindow) {
          return clients.openWindow(event.notification.data.url || '/');
        }
      })
  );
});

self.addEventListener('install', function(event) {
  console.log('Service Worker 설치됨');
  self.skipWaiting();
});

self.addEventListener('activate', function(event) {
  console.log('Service Worker 활성화됨');
  event.waitUntil(clients.claim());
});
