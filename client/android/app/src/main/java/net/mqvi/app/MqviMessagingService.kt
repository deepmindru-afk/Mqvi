package net.mqvi.app

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import com.capacitorjs.plugins.pushnotifications.MessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * Intercepts incoming-call data messages to show a full-screen incoming-call
 * notification — so a killed app still rings over the lock screen — and delegates
 * every other message (DM notifications, token refresh) to the Capacitor base
 * service via super. The base MessagingService drives token registration and the
 * pushNotificationReceived event, so DMs and token sync keep working unchanged.
 *
 * Registered in AndroidManifest in place of the plugin's MessagingService (only one
 * FirebaseMessagingService can win the MESSAGING_EVENT intent).
 */
class MqviMessagingService : MessagingService() {

    override fun onMessageReceived(remoteMessage: RemoteMessage) {
        if (remoteMessage.data["type"] == "call") {
            showIncomingCall(remoteMessage.data)
            return // handled natively — do not let Capacitor fire pushNotificationReceived
        }
        super.onMessageReceived(remoteMessage)
    }

    private fun showIncomingCall(data: Map<String, String>) {
        ensureCallChannel()

        val callId = data["call_id"].orEmpty()
        val title = data["title"] ?: getString(R.string.app_name)
        val body = data["body"].orEmpty()

        // Launch the app for the call; the in-app overlay handles answer/decline once
        // the WebSocket reconnects and the server re-delivers the still-ringing call.
        val launch = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP
            putExtra(MainActivity.EXTRA_INCOMING_CALL, true)
            putExtra(MainActivity.EXTRA_CALL_ID, callId)
        }
        val pi = PendingIntent.getActivity(
            this,
            callId.hashCode(),
            launch,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val notification = NotificationCompat.Builder(this, CALLS_CHANNEL)
            .setSmallIcon(android.R.drawable.sym_call_incoming)
            .setContentTitle(title)
            .setContentText(body)
            .setPriority(NotificationCompat.PRIORITY_MAX)
            .setCategory(NotificationCompat.CATEGORY_CALL)
            .setAutoCancel(true)
            // Mirror the server's 60s ring timeout so a missed call's notification clears.
            .setTimeoutAfter(60_000)
            .setContentIntent(pi)
            .setFullScreenIntent(pi, true)
            .build()

        (getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager)
            .notify(callId.hashCode(), notification)
    }

    // Ensures the "calls" channel exists for the killed-app case where the JS
    // createChannel hasn't run this session. Matches the channel id the FCM payload
    // and the JS hook use.
    private fun ensureCallChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (nm.getNotificationChannel(CALLS_CHANNEL) != null) return
        val channel = NotificationChannel(
            CALLS_CHANNEL,
            "Calls",
            NotificationManager.IMPORTANCE_HIGH,
        ).apply {
            description = "Incoming calls"
            enableVibration(true)
        }
        nm.createNotificationChannel(channel)
    }

    companion object {
        private const val CALLS_CHANNEL = "calls"
    }
}
