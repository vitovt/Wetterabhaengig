package com.vitovt.wetterabhaengig.bg;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.os.Build;
import android.os.IBinder;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.Locale;
import java.util.concurrent.atomic.AtomicBoolean;

public final class BackgroundCheckService extends Service {
	private static final int FOREGROUND_ID = 12001;
	private static final String FOREGROUND_CHANNEL_ID = "wetterabhaengig_background_service";
	private static final String FOREGROUND_CHANNEL_NAME = "Wetterabhaengig Background";

	private static final String ALERT_CHANNEL_ID = "wetterabhaengig_alerts";
	private static final String ALERT_CHANNEL_NAME = "Wetterabhaengig Alerts";

	private static final AtomicBoolean RUNNING = new AtomicBoolean(false);

	public static void startNow(Context context) {
		Intent intent = new Intent(context, BackgroundCheckService.class);
		try {
			if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
				context.startForegroundService(intent);
			} else {
				context.startService(intent);
			}
		} catch (Throwable ignored) {
		}
	}

	public static void stopNow(Context context) {
		Intent intent = new Intent(context, BackgroundCheckService.class);
		try {
			context.stopService(intent);
		} catch (Throwable ignored) {
		}
	}

	@Override
	public int onStartCommand(Intent intent, int flags, int startId) {
		SharedPreferences prefs = BackgroundBridge.prefs(this);
		if (!prefs.getBoolean(BackgroundBridge.KEY_ENABLED, false)) {
			stopSelfResult(startId);
			return START_NOT_STICKY;
		}

		ensureForegroundChannel();
		startForeground(FOREGROUND_ID, foregroundNotification());

		if (!RUNNING.compareAndSet(false, true)) {
			stopForeground(true);
			stopSelfResult(startId);
			return START_NOT_STICKY;
		}

		final Context appCtx = getApplicationContext();
		new Thread(new Runnable() {
			@Override
			public void run() {
				try {
					runCheck(appCtx);
				} finally {
					RUNNING.set(false);
					stopForeground(true);
					stopSelfResult(startId);
				}
			}
		}, "wetter-bg-check").start();

		return START_NOT_STICKY;
	}

	@Override
	public IBinder onBind(Intent intent) {
		return null;
	}

	private void runCheck(Context context) {
		SharedPreferences prefs = BackgroundBridge.prefs(context);
		if (!prefs.getBoolean(BackgroundBridge.KEY_ENABLED, false)) {
			BackgroundCheckReceiver.cancel(context);
			return;
		}

		int periodMinutes = Math.max(15, prefs.getInt(BackgroundBridge.KEY_PERIOD_MINUTES, 60));
		boolean notificationsEnabled = prefs.getBoolean(BackgroundBridge.KEY_NOTIFICATIONS_ENABLED, true);

		double lat = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_LAT, 52.52);
		double lon = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_LON, 13.405);

		double pressureMedium = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_PRESSURE_MEDIUM, 5.0);
		double pressureHigh = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_PRESSURE_HIGH, 8.0);
		double pressureCritical = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_PRESSURE_CRITICAL, 12.0);
		double kMedium = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_K_MEDIUM, 4.0);
		double kHigh = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_K_HIGH, 5.0);
		double kCritical = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_K_CRITICAL, 6.0);

		double pressureDelta = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_LAST_PRESSURE_DELTA, 0.0);
		double kIndex = BackgroundBridge.getDouble(prefs, BackgroundBridge.KEY_LAST_K_INDEX, 0.0);

		boolean pressureOk = false;
		boolean kOk = false;
		try {
			pressureDelta = fetchPressureDelta(lat, lon);
			pressureOk = true;
		} catch (Exception ignored) {
		}
		try {
			kIndex = fetchLatestKIndex();
			kOk = true;
		} catch (Exception ignored) {
		}

		if (!pressureOk && !kOk) {
			BackgroundCheckReceiver.scheduleNext(context, periodMinutes * 60L * 1000L);
			return;
		}

		int pressureRisk = riskFromPressure(pressureDelta, pressureMedium, pressureHigh, pressureCritical);
		int kRisk = riskFromK(kIndex, kMedium, kHigh, kCritical);
		int overallRisk = Math.max(pressureRisk, kRisk);

		int previousRisk = prefs.getInt(BackgroundBridge.KEY_LAST_RISK, -1);
		SharedPreferences.Editor editor = prefs.edit();
		BackgroundBridge.putDouble(editor, BackgroundBridge.KEY_LAST_PRESSURE_DELTA, pressureDelta);
		BackgroundBridge.putDouble(editor, BackgroundBridge.KEY_LAST_K_INDEX, kIndex);
		editor.putInt(BackgroundBridge.KEY_LAST_RISK, overallRisk);
		editor.putLong(BackgroundBridge.KEY_LAST_CHECK_MS, System.currentTimeMillis());
		editor.apply();

		if (notificationsEnabled && previousRisk >= 0 && previousRisk != overallRisk) {
			showStateChangedNotification(previousRisk, overallRisk);
		}

		BackgroundCheckReceiver.scheduleNext(context, periodMinutes * 60L * 1000L);
	}

	private double fetchPressureDelta(double lat, double lon) throws Exception {
		String url = String.format(
			Locale.US,
			"https://api.open-meteo.com/v1/forecast?latitude=%.5f&longitude=%.5f&hourly=surface_pressure&forecast_days=2&timezone=auto",
			lat,
			lon
		);
		String payload = readUrl(url);
		JSONObject root = new JSONObject(payload);
		JSONObject hourly = root.getJSONObject("hourly");
		JSONArray values = hourly.getJSONArray("surface_pressure");
		if (values.length() < 24) {
			throw new IllegalStateException("not enough pressure points");
		}
		double min = values.getDouble(0);
		double max = values.getDouble(0);
		for (int i = 0; i < 24; i++) {
			double value = values.getDouble(i);
			if (value < min) {
				min = value;
			}
			if (value > max) {
				max = value;
			}
		}
		return max - min;
	}

	private double fetchLatestKIndex() throws Exception {
		String payload = readUrl("https://services.swpc.noaa.gov/json/planetary_k_index_1m.json");
		JSONArray rows = new JSONArray(payload);
		if (rows.length() == 0) {
			throw new IllegalStateException("empty k-index payload");
		}
		JSONObject last = rows.getJSONObject(rows.length() - 1);
		String[] keys = new String[]{"k_index", "kp_index", "kp", "kIndex"};
		for (String key : keys) {
			if (!last.has(key)) {
				continue;
			}
			Object raw = last.get(key);
			Double value = asDouble(raw);
			if (value != null) {
				return value;
			}
		}
		throw new IllegalStateException("k-index field not found");
	}

	private static Double asDouble(Object raw) {
		if (raw == null) {
			return null;
		}
		if (raw instanceof Number) {
			return ((Number) raw).doubleValue();
		}
		if (raw instanceof String) {
			try {
				return Double.parseDouble((String) raw);
			} catch (Exception ignored) {
				return null;
			}
		}
		return null;
	}

	private static int riskFromPressure(double value, double medium, double high, double critical) {
		if (value > critical) {
			return 3;
		}
		if (value > high) {
			return 2;
		}
		if (value > medium) {
			return 1;
		}
		return 0;
	}

	private static int riskFromK(double value, double medium, double high, double critical) {
		if (value >= critical) {
			return 3;
		}
		if (value >= high) {
			return 2;
		}
		if (value >= medium) {
			return 1;
		}
		return 0;
	}

	private void showStateChangedNotification(int previousRisk, int currentRisk) {
		NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
		if (manager == null) {
			return;
		}
		ensureAlertChannel(manager);

		String text = "State changed: " + riskLabel(previousRisk) + " -> " + riskLabel(currentRisk);
		Notification.Builder builder;
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
			builder = new Notification.Builder(this, ALERT_CHANNEL_ID);
		} else {
			builder = new Notification.Builder(this);
		}
		builder
			.setSmallIcon(resolveSmallIcon())
			.setContentTitle("Wetterabhaengig")
			.setContentText(text)
			.setAutoCancel(true);
		manager.notify((int) (System.currentTimeMillis() & 0x7fffffff), builder.build());
	}

	private Notification foregroundNotification() {
		Notification.Builder builder;
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
			builder = new Notification.Builder(this, FOREGROUND_CHANNEL_ID);
		} else {
			builder = new Notification.Builder(this);
		}
		return builder
			.setSmallIcon(resolveSmallIcon())
			.setContentTitle("Wetterabhaengig")
			.setContentText("Background weather check in progress")
			.setOngoing(true)
			.setOnlyAlertOnce(true)
			.build();
	}

	private int resolveSmallIcon() {
		int icon = getResources().getIdentifier("ic_stat_wetterabhaengig", "drawable", getPackageName());
		if (icon != 0) {
			return icon;
		}
		return android.R.drawable.ic_dialog_info;
	}

	private void ensureForegroundChannel() {
		if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
			return;
		}
		NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
		if (manager == null) {
			return;
		}
		NotificationChannel channel = new NotificationChannel(
			FOREGROUND_CHANNEL_ID,
			FOREGROUND_CHANNEL_NAME,
			NotificationManager.IMPORTANCE_LOW
		);
		channel.setDescription("Runs periodic weather checks when the app is closed.");
		manager.createNotificationChannel(channel);
	}

	private void ensureAlertChannel(NotificationManager manager) {
		if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
			return;
		}
		NotificationChannel channel = new NotificationChannel(
			ALERT_CHANNEL_ID,
			ALERT_CHANNEL_NAME,
			NotificationManager.IMPORTANCE_DEFAULT
		);
		channel.setDescription("Risk change notifications");
		manager.createNotificationChannel(channel);
	}

	private static String riskLabel(int level) {
		switch (level) {
			case 0:
				return "LOW";
			case 1:
				return "MEDIUM";
			case 2:
				return "HIGH";
			case 3:
				return "CRITICAL";
			default:
				return "UNKNOWN";
		}
	}

	private static String readUrl(String rawUrl) throws Exception {
		HttpURLConnection connection = null;
		InputStream input = null;
		try {
			connection = (HttpURLConnection) new URL(rawUrl).openConnection();
			connection.setConnectTimeout(10000);
			connection.setReadTimeout(10000);
			connection.setRequestMethod("GET");

			int status = connection.getResponseCode();
			if (status != HttpURLConnection.HTTP_OK) {
				throw new IllegalStateException("unexpected HTTP status: " + status);
			}

			input = connection.getInputStream();
			ByteArrayOutputStream out = new ByteArrayOutputStream();
			byte[] buf = new byte[4096];
			int read;
			while ((read = input.read(buf)) != -1) {
				out.write(buf, 0, read);
			}
			return out.toString(StandardCharsets.UTF_8.name());
		} finally {
			if (input != null) {
				try {
					input.close();
				} catch (Exception ignored) {
				}
			}
			if (connection != null) {
				connection.disconnect();
			}
		}
	}
}
