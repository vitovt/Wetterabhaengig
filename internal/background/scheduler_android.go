//go:build android

package background

/*
#include <jni.h>
#include <stdlib.h>

static jint jni_GetEnv(JavaVM *vm, JNIEnv **env, jint version) {
	return (*vm)->GetEnv(vm, (void**)env, version);
}
static jint jni_AttachCurrentThread(JavaVM *vm, JNIEnv **p_env, void *thr_args) {
	return (*vm)->AttachCurrentThread(vm, p_env, thr_args);
}
static jint jni_DetachCurrentThread(JavaVM *vm) {
	return (*vm)->DetachCurrentThread(vm);
}
static jclass jni_FindClass(JNIEnv *env, const char *name) {
	return (*env)->FindClass(env, name);
}
static jmethodID jni_GetStaticMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetStaticMethodID(env, clazz, name, sig);
}
static void jni_CallStaticVoidMethodCtx(JNIEnv *env, jclass clazz, jmethodID methodID, jobject ctx) {
	(*env)->CallStaticVoidMethod(env, clazz, methodID, ctx);
}
static void jni_CallStaticVoidMethodConfig(
	JNIEnv *env,
	jclass clazz,
	jmethodID methodID,
	jobject ctx,
	jboolean enabled,
	jint period,
	jboolean notifications,
	jdouble lat,
	jdouble lon,
	jdouble pressureMedium,
	jdouble pressureHigh,
	jdouble pressureCritical,
	jdouble kMedium,
	jdouble kHigh,
	jdouble kCritical
) {
	(*env)->CallStaticVoidMethod(
		env,
		clazz,
		methodID,
		ctx,
		enabled,
		period,
		notifications,
		lat,
		lon,
		pressureMedium,
		pressureHigh,
		pressureCritical,
		kMedium,
		kHigh,
		kCritical
	);
}
static jthrowable jni_ExceptionOccurred(JNIEnv *env) {
	return (*env)->ExceptionOccurred(env);
}
static void jni_ExceptionClear(JNIEnv *env) {
	(*env)->ExceptionClear(env);
}
static void jni_DeleteLocalRef(JNIEnv *env, jobject obj) {
	if (obj != NULL) {
		(*env)->DeleteLocalRef(env, obj);
	}
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"gioui.org/app"

	"github.com/vitovt/wetterabhaengig/internal/domain"
)

const (
	bridgeClassName   = "com/vitovt/wetterabhaengig/bg/BackgroundBridge"
	minSchedulePeriod = 15
)

func syncConfig(cfg domain.AppConfig, lat, lon float64) error {
	return withJNIEnv(func(env *C.JNIEnv, appCtx C.jobject) error {
		bridgeClass, err := findBridgeClass(env)
		if err != nil {
			return err
		}
		defer C.jni_DeleteLocalRef(env, C.jobject(bridgeClass))

		updateMethod, err := getStaticMethodID(
			env,
			bridgeClass,
			"updateConfig",
			"(Landroid/content/Context;ZIZDDDDDDDD)V",
		)
		if err != nil {
			return err
		}

		enabled := C.jboolean(C.JNI_FALSE)
		if cfg.Schedule.RunWhenClosed {
			enabled = C.jboolean(C.JNI_TRUE)
		}
		notifications := C.jboolean(C.JNI_FALSE)
		if cfg.Notifications.Enabled {
			notifications = C.jboolean(C.JNI_TRUE)
		}

		period := cfg.Schedule.PeriodMinutes
		if period < cfg.Schedule.MinMinutes {
			period = cfg.Schedule.MinMinutes
		}
		if period < minSchedulePeriod {
			period = minSchedulePeriod
		}

		C.jni_CallStaticVoidMethodConfig(
			env,
			bridgeClass,
			updateMethod,
			appCtx,
			enabled,
			C.jint(period),
			notifications,
			C.jdouble(lat),
			C.jdouble(lon),
			C.jdouble(cfg.Pressure.Medium),
			C.jdouble(cfg.Pressure.High),
			C.jdouble(cfg.Pressure.Crit),
			C.jdouble(cfg.KIndex.Medium),
			C.jdouble(cfg.KIndex.High),
			C.jdouble(cfg.KIndex.Crit),
		)
		return jniException(env, "BackgroundBridge.updateConfig")
	})
}

func triggerNow() error {
	return withJNIEnv(func(env *C.JNIEnv, appCtx C.jobject) error {
		bridgeClass, err := findBridgeClass(env)
		if err != nil {
			return err
		}
		defer C.jni_DeleteLocalRef(env, C.jobject(bridgeClass))

		startMethod, err := getStaticMethodID(
			env,
			bridgeClass,
			"startNow",
			"(Landroid/content/Context;)V",
		)
		if err != nil {
			return err
		}
		C.jni_CallStaticVoidMethodCtx(env, bridgeClass, startMethod, appCtx)
		return jniException(env, "BackgroundBridge.startNow")
	})
}

func withJNIEnv(fn func(env *C.JNIEnv, appCtx C.jobject) error) error {
	jvmPtr := app.JavaVM()
	if jvmPtr == 0 {
		return errors.New("android JVM is not initialized")
	}
	appCtxPtr := app.AppContext()
	if appCtxPtr == 0 {
		return errors.New("android app context is unavailable")
	}

	jvm := (*C.JavaVM)(unsafe.Pointer(jvmPtr))
	appCtx := C.jobject(unsafe.Pointer(appCtxPtr))

	var env *C.JNIEnv
	res := C.jni_GetEnv(jvm, &env, C.JNI_VERSION_1_6)
	detach := false
	if res != C.JNI_OK {
		if res != C.JNI_EDETACHED {
			return fmt.Errorf("JNI GetEnv failed: %d", int(res))
		}
		if C.jni_AttachCurrentThread(jvm, &env, nil) != C.JNI_OK {
			return errors.New("JNI attach thread failed")
		}
		detach = true
	}
	if detach {
		defer C.jni_DetachCurrentThread(jvm)
	}
	return fn(env, appCtx)
}

func findBridgeClass(env *C.JNIEnv) (C.jclass, error) {
	cClassName := C.CString(bridgeClassName)
	defer C.free(unsafe.Pointer(cClassName))

	bridgeClass := C.jni_FindClass(env, cClassName)
	var zeroClass C.jclass
	if bridgeClass == zeroClass {
		if err := jniException(env, "FindClass:"+bridgeClassName); err != nil {
			return zeroClass, err
		}
		return zeroClass, fmt.Errorf("android class not found: %s", bridgeClassName)
	}
	if err := jniException(env, "FindClass:"+bridgeClassName); err != nil {
		return zeroClass, err
	}
	return bridgeClass, nil
}

func getStaticMethodID(env *C.JNIEnv, class C.jclass, name, sig string) (C.jmethodID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSig := C.CString(sig)
	defer C.free(unsafe.Pointer(cSig))

	method := C.jni_GetStaticMethodID(env, class, cName, cSig)
	var zeroMethod C.jmethodID
	if method == zeroMethod {
		return zeroMethod, fmt.Errorf("android static method not found: %s %s", name, sig)
	}
	if err := jniException(env, "GetStaticMethodID:"+name); err != nil {
		return zeroMethod, err
	}
	return method, nil
}

func jniException(env *C.JNIEnv, where string) error {
	exc := C.jni_ExceptionOccurred(env)
	var zeroExc C.jthrowable
	if exc == zeroExc {
		return nil
	}
	C.jni_ExceptionClear(env)
	C.jni_DeleteLocalRef(env, C.jobject(exc))
	return fmt.Errorf("android JNI exception at %s", where)
}
