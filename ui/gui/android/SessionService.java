package com.bedrocktool.android;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;

// SessionService is a foreground service that runs while Bedrocktool is
// connected to a server or replaying a capture. While it runs, Android
// shows a notification and doesn't kill the app when it's in the background
// (for example while Minecraft is open); its wake and Wi-Fi locks keep the
// connection going with the screen off.
public final class SessionService extends Service {
    static final String EXTRA_TITLE = "title";
    static final String EXTRA_TEXT = "text";
    private static final String CHANNEL = "session";
    private static final int ID = 1;
    // ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE (API 34)
    private static final int TYPE_SPECIAL_USE = 0x40000000;

    private PowerManager.WakeLock wake;
    private WifiManager.WifiLock wifi;

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        String title = intent != null ? intent.getStringExtra(EXTRA_TITLE) : null;
        String text = intent != null ? intent.getStringExtra(EXTRA_TEXT) : null;
        Notification n = notification(title != null ? title : "Bedrocktool", text != null ? text : "");
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(ID, n, TYPE_SPECIAL_USE);
        } else {
            startForeground(ID, n);
        }
        lock();
        // if Android kills it anyway, the session is gone: don't restart empty
        return START_NOT_STICKY;
    }

    private Notification notification(String title, String text) {
        Notification.Builder b;
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            NotificationChannel ch = new NotificationChannel(CHANNEL, title, NotificationManager.IMPORTANCE_LOW);
            ch.setShowBadge(false);
            nm.createNotificationChannel(ch);
            b = new Notification.Builder(this, CHANNEL);
        } else {
            b = new Notification.Builder(this);
        }
        if (Build.VERSION.SDK_INT >= 31) {
            // Android 12+ holds a service's notification back for up to 10s
            b.setForegroundServiceBehavior(Notification.FOREGROUND_SERVICE_IMMEDIATE);
        }
        b.setContentTitle(title)
                .setContentText(text)
                .setSmallIcon(getApplicationInfo().icon)
                .setOngoing(true);
        Intent open = getPackageManager().getLaunchIntentForPackage(getPackageName());
        if (open != null) {
            int f = Build.VERSION.SDK_INT >= 23 ? PendingIntent.FLAG_IMMUTABLE : 0;
            b.setContentIntent(PendingIntent.getActivity(this, 0, open, f));
        }
        return b.build();
    }

    private void lock() {
        if (wake == null) {
            PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
            wake = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "bedrocktool:session");
            wake.setReferenceCounted(false);
            wake.acquire();
        }
        if (wifi == null) {
            WifiManager wm = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
            if (wm != null) {
                @SuppressWarnings("deprecation")
                int mode = WifiManager.WIFI_MODE_FULL_HIGH_PERF;
                wifi = wm.createWifiLock(mode, "bedrocktool:session");
                wifi.setReferenceCounted(false);
                wifi.acquire();
            }
        }
    }

    @Override
    public void onDestroy() {
        if (wake != null) {
            wake.release();
            wake = null;
        }
        if (wifi != null) {
            wifi.release();
            wifi = null;
        }
        super.onDestroy();
    }
}
