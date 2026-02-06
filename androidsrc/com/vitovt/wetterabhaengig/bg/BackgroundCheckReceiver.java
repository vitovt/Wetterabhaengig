package com.vitovt.wetterabhaengig.bg;

import android.app.AlarmManager;
import android.app.PendingIntent;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.os.Build;

public final class BackgroundCheckReceiver extends BroadcastReceiver {
	private static final String ACTION_RUN = "com.vitovt.wetterabhaengig.BG_RUN";
	private static final int REQUEST_CODE = 9071;
	private static final long MIN_DELAY_MS = 60L * 1000L;

	@Override
	public void onReceive(Context context, Intent intent) {
		Context appCtx = context.getApplicationContext();
		if (!BackgroundBridge.prefs(appCtx).getBoolean(BackgroundBridge.KEY_ENABLED, false)) {
			cancel(appCtx);
			return;
		}
		BackgroundCheckService.startNow(appCtx);
	}

	static void scheduleNext(Context context, long delayMillis) {
		AlarmManager alarmManager = (AlarmManager) context.getSystemService(Context.ALARM_SERVICE);
		if (alarmManager == null) {
			return;
		}
		long boundedDelay = Math.max(delayMillis, MIN_DELAY_MS);
		long triggerAt = System.currentTimeMillis() + boundedDelay;
		PendingIntent pendingIntent = pendingIntent(context, PendingIntent.FLAG_UPDATE_CURRENT | immutableFlag());
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
			alarmManager.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, triggerAt, pendingIntent);
		} else {
			alarmManager.set(AlarmManager.RTC_WAKEUP, triggerAt, pendingIntent);
		}
	}

	static void cancel(Context context) {
		AlarmManager alarmManager = (AlarmManager) context.getSystemService(Context.ALARM_SERVICE);
		if (alarmManager == null) {
			return;
		}
		PendingIntent pendingIntent = pendingIntent(context, PendingIntent.FLAG_UPDATE_CURRENT | immutableFlag());
		alarmManager.cancel(pendingIntent);
		pendingIntent.cancel();
	}

	private static PendingIntent pendingIntent(Context context, int flags) {
		Intent intent = new Intent(context, BackgroundCheckReceiver.class);
		intent.setAction(ACTION_RUN);
		return PendingIntent.getBroadcast(context, REQUEST_CODE, intent, flags);
	}

	private static int immutableFlag() {
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
			return PendingIntent.FLAG_IMMUTABLE;
		}
		return 0;
	}
}
