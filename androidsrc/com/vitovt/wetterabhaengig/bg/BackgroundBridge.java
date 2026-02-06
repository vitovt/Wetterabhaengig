package com.vitovt.wetterabhaengig.bg;

import android.content.Context;
import android.content.SharedPreferences;

public final class BackgroundBridge {
	static final String PREFS_NAME = "wetterabhaengig_bg";

	static final String KEY_ENABLED = "enabled";
	static final String KEY_PERIOD_MINUTES = "period_minutes";
	static final String KEY_NOTIFICATIONS_ENABLED = "notifications_enabled";
	static final String KEY_LAT = "lat";
	static final String KEY_LON = "lon";
	static final String KEY_PRESSURE_MEDIUM = "pressure_medium";
	static final String KEY_PRESSURE_HIGH = "pressure_high";
	static final String KEY_PRESSURE_CRITICAL = "pressure_critical";
	static final String KEY_K_MEDIUM = "k_medium";
	static final String KEY_K_HIGH = "k_high";
	static final String KEY_K_CRITICAL = "k_critical";
	static final String KEY_LAST_RISK = "last_risk";
	static final String KEY_LAST_PRESSURE_DELTA = "last_pressure_delta";
	static final String KEY_LAST_K_INDEX = "last_k_index";
	static final String KEY_LAST_CHECK_MS = "last_check_ms";

	private static final int MIN_PERIOD_MINUTES = 15;

	private BackgroundBridge() {
	}

	static SharedPreferences prefs(Context context) {
		return context.getApplicationContext().getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
	}

	public static void updateConfig(
		Context context,
		boolean enabled,
		int periodMinutes,
		boolean notificationsEnabled,
		double lat,
		double lon,
		double pressureMedium,
		double pressureHigh,
		double pressureCritical,
		double kMedium,
		double kHigh,
		double kCritical
	) {
		Context appCtx = context.getApplicationContext();
		int boundedPeriod = Math.max(MIN_PERIOD_MINUTES, periodMinutes);

		SharedPreferences.Editor editor = prefs(appCtx).edit();
		editor.putBoolean(KEY_ENABLED, enabled);
		editor.putInt(KEY_PERIOD_MINUTES, boundedPeriod);
		editor.putBoolean(KEY_NOTIFICATIONS_ENABLED, notificationsEnabled);
		putDouble(editor, KEY_LAT, lat);
		putDouble(editor, KEY_LON, lon);
		putDouble(editor, KEY_PRESSURE_MEDIUM, pressureMedium);
		putDouble(editor, KEY_PRESSURE_HIGH, pressureHigh);
		putDouble(editor, KEY_PRESSURE_CRITICAL, pressureCritical);
		putDouble(editor, KEY_K_MEDIUM, kMedium);
		putDouble(editor, KEY_K_HIGH, kHigh);
		putDouble(editor, KEY_K_CRITICAL, kCritical);
		editor.apply();

		if (!enabled) {
			BackgroundCheckReceiver.cancel(appCtx);
			BackgroundCheckService.stopNow(appCtx);
			return;
		}

		BackgroundCheckReceiver.scheduleNext(appCtx, boundedPeriod * 60L * 1000L);
	}

	public static void startNow(Context context) {
		Context appCtx = context.getApplicationContext();
		if (!prefs(appCtx).getBoolean(KEY_ENABLED, false)) {
			return;
		}
		BackgroundCheckService.startNow(appCtx);
	}

	static void putDouble(SharedPreferences.Editor editor, String key, double value) {
		editor.putLong(key, Double.doubleToRawLongBits(value));
	}

	static double getDouble(SharedPreferences prefs, String key, double defaultValue) {
		long fallback = Double.doubleToRawLongBits(defaultValue);
		return Double.longBitsToDouble(prefs.getLong(key, fallback));
	}
}
