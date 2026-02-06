//go:build android

package gps

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
static jclass jni_GetObjectClass(JNIEnv *env, jobject obj) {
	return (*env)->GetObjectClass(env, obj);
}
static jmethodID jni_GetMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetMethodID(env, clazz, name, sig);
}
static jint jni_CallIntMethodStr(JNIEnv *env, jobject obj, jmethodID methodID, jstring str) {
	return (*env)->CallIntMethod(env, obj, methodID, str);
}
static jobject jni_CallObjectMethodStr(JNIEnv *env, jobject obj, jmethodID methodID, jstring str) {
	return (*env)->CallObjectMethod(env, obj, methodID, str);
}
static jobject jni_CallObjectMethodNoArgs(JNIEnv *env, jobject obj, jmethodID methodID) {
	return (*env)->CallObjectMethod(env, obj, methodID);
}
static jdouble jni_CallDoubleMethodNoArgs(JNIEnv *env, jobject obj, jmethodID methodID) {
	return (*env)->CallDoubleMethod(env, obj, methodID);
}
static jfloat jni_CallFloatMethodNoArgs(JNIEnv *env, jobject obj, jmethodID methodID) {
	return (*env)->CallFloatMethod(env, obj, methodID);
}
static void jni_CallVoidMethodPermReq(JNIEnv *env, jobject obj, jmethodID methodID, jobjectArray perms, jint reqCode) {
	(*env)->CallVoidMethod(env, obj, methodID, perms, reqCode);
}
static jstring jni_NewStringUTF(JNIEnv *env, const char *chars) {
	return (*env)->NewStringUTF(env, chars);
}
static jobjectArray jni_NewObjectArray(JNIEnv *env, jsize len, jclass clazz, jobject init) {
	return (*env)->NewObjectArray(env, len, clazz, init);
}
static void jni_SetObjectArrayElement(JNIEnv *env, jobjectArray array, jsize index, jobject value) {
	(*env)->SetObjectArrayElement(env, array, index, value);
}
static jboolean jni_IsInstanceOf(JNIEnv *env, jobject obj, jclass clazz) {
	return (*env)->IsInstanceOf(env, obj, clazz);
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
	"context"
	"errors"
	"fmt"
	"unsafe"

	"gioui.org/app"
)

const (
	permissionGranted         = 0
	locationPermissionRequest = 4207
)

type androidProvider struct{}

func newProvider() Provider {
	return &androidProvider{}
}

func (a *androidProvider) CurrentLocation(ctx context.Context) (float64, float64, error) {
	_ = ctx
	var lat float64
	var lon float64

	err := withJNIEnv(func(env *C.JNIEnv, appCtx C.jobject) error {
		granted, err := checkFineLocationPermission(env, appCtx)
		if err != nil {
			return err
		}
		if !granted {
			_ = requestFineLocationPermission(env, appCtx)
			return errors.New("location permission is not granted yet; allow it and press the GPS button again")
		}

		manager, err := getLocationManager(env, appCtx)
		if err != nil {
			return err
		}
		defer C.jni_DeleteLocalRef(env, manager)

		fixLat, fixLon, err := getBestLastKnownLocation(env, manager)
		if err != nil {
			return err
		}
		lat = fixLat
		lon = fixLon
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return lat, lon, nil
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

func checkFineLocationPermission(env *C.JNIEnv, appCtx C.jobject) (bool, error) {
	ctxClass := C.jni_GetObjectClass(env, appCtx)
	var zeroClass C.jclass
	if ctxClass == zeroClass {
		return false, errors.New("cannot get Android context class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(ctxClass))

	method, err := getMethodID(env, ctxClass, "checkSelfPermission", "(Ljava/lang/String;)I")
	if err != nil {
		return false, err
	}

	perm, err := newJavaString(env, "android.permission.ACCESS_FINE_LOCATION")
	if err != nil {
		return false, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(perm))

	res := C.jni_CallIntMethodStr(env, appCtx, method, perm)
	if err := jniException(env, "checkSelfPermission"); err != nil {
		return false, err
	}
	return int(res) == permissionGranted, nil
}

func requestFineLocationPermission(env *C.JNIEnv, appCtx C.jobject) error {
	activityClassName := C.CString("android/app/Activity")
	defer C.free(unsafe.Pointer(activityClassName))
	activityClass := C.jni_FindClass(env, activityClassName)
	var zeroClass C.jclass
	if activityClass == zeroClass {
		return errors.New("cannot resolve android.app.Activity class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(activityClass))
	if C.jni_IsInstanceOf(env, appCtx, activityClass) != C.JNI_TRUE {
		return errors.New("app context is not an Activity; cannot show permission dialog")
	}

	ctxClass := C.jni_GetObjectClass(env, appCtx)
	if ctxClass == zeroClass {
		return errors.New("cannot get context class for permission request")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(ctxClass))

	requestMethod, err := getMethodID(env, ctxClass, "requestPermissions", "([Ljava/lang/String;I)V")
	if err != nil {
		return err
	}

	stringClassName := C.CString("java/lang/String")
	defer C.free(unsafe.Pointer(stringClassName))
	stringClass := C.jni_FindClass(env, stringClassName)
	if stringClass == zeroClass {
		return errors.New("cannot resolve java.lang.String class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(stringClass))

	var zeroObj C.jobject
	perms := C.jni_NewObjectArray(env, 1, stringClass, zeroObj)
	var zeroArray C.jobjectArray
	if perms == zeroArray {
		return errors.New("cannot create permission array")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(perms))

	perm, err := newJavaString(env, "android.permission.ACCESS_FINE_LOCATION")
	if err != nil {
		return err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(perm))

	C.jni_SetObjectArrayElement(env, perms, 0, C.jobject(perm))
	if err := jniException(env, "SetObjectArrayElement"); err != nil {
		return err
	}

	C.jni_CallVoidMethodPermReq(env, appCtx, requestMethod, perms, C.jint(locationPermissionRequest))
	return jniException(env, "requestPermissions")
}

func getLocationManager(env *C.JNIEnv, appCtx C.jobject) (C.jobject, error) {
	ctxClass := C.jni_GetObjectClass(env, appCtx)
	var zeroClass C.jclass
	var zeroObj C.jobject
	if ctxClass == zeroClass {
		return zeroObj, errors.New("cannot get context class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(ctxClass))

	getSystemServiceMethod, err := getMethodID(env, ctxClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;")
	if err != nil {
		return zeroObj, err
	}

	locationService, err := newJavaString(env, "location")
	if err != nil {
		return zeroObj, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(locationService))

	manager := C.jni_CallObjectMethodStr(env, appCtx, getSystemServiceMethod, locationService)
	if err := jniException(env, "getSystemService"); err != nil {
		return zeroObj, err
	}
	if manager == zeroObj {
		return zeroObj, errors.New("location manager is unavailable")
	}
	return manager, nil
}

func getBestLastKnownLocation(env *C.JNIEnv, locationManager C.jobject) (float64, float64, error) {
	managerClass := C.jni_GetObjectClass(env, locationManager)
	var zeroClass C.jclass
	if managerClass == zeroClass {
		return 0, 0, errors.New("cannot get LocationManager class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(managerClass))

	getLastKnownLocationMethod, err := getMethodID(env, managerClass, "getLastKnownLocation", "(Ljava/lang/String;)Landroid/location/Location;")
	if err != nil {
		return 0, 0, err
	}

	providers := []string{"gps", "network", "passive"}
	for _, provider := range providers {
		p, err := newJavaString(env, provider)
		if err != nil {
			return 0, 0, err
		}
		locationObj := C.jni_CallObjectMethodStr(env, locationManager, getLastKnownLocationMethod, p)
		C.jni_DeleteLocalRef(env, C.jobject(p))
		if err := jniException(env, "getLastKnownLocation"); err != nil {
			continue
		}
		var zeroObj C.jobject
		if locationObj == zeroObj {
			continue
		}

		lat, lon, err := parseLocation(env, locationObj)
		C.jni_DeleteLocalRef(env, locationObj)
		if err == nil {
			return lat, lon, nil
		}
	}

	return 0, 0, errors.New("no GPS/network location fix is available yet")
}

func parseLocation(env *C.JNIEnv, locationObj C.jobject) (float64, float64, error) {
	locationClass := C.jni_GetObjectClass(env, locationObj)
	var zeroClass C.jclass
	if locationClass == zeroClass {
		return 0, 0, errors.New("cannot get Location class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(locationClass))

	getLatitudeMethod, err := getMethodID(env, locationClass, "getLatitude", "()D")
	if err != nil {
		return 0, 0, err
	}
	getLongitudeMethod, err := getMethodID(env, locationClass, "getLongitude", "()D")
	if err != nil {
		return 0, 0, err
	}

	lat := float64(C.jni_CallDoubleMethodNoArgs(env, locationObj, getLatitudeMethod))
	if err := jniException(env, "getLatitude"); err != nil {
		return 0, 0, err
	}
	lon := float64(C.jni_CallDoubleMethodNoArgs(env, locationObj, getLongitudeMethod))
	if err := jniException(env, "getLongitude"); err != nil {
		return 0, 0, err
	}
	return lat, lon, nil
}

func getMethodID(env *C.JNIEnv, class C.jclass, name, sig string) (C.jmethodID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSig := C.CString(sig)
	defer C.free(unsafe.Pointer(cSig))

	method := C.jni_GetMethodID(env, class, cName, cSig)
	var zeroMethod C.jmethodID
	if method == zeroMethod {
		return zeroMethod, fmt.Errorf("android method not found: %s %s", name, sig)
	}
	if err := jniException(env, "GetMethodID:"+name); err != nil {
		return zeroMethod, err
	}
	return method, nil
}

func newJavaString(env *C.JNIEnv, value string) (C.jstring, error) {
	cVal := C.CString(value)
	defer C.free(unsafe.Pointer(cVal))
	jstr := C.jni_NewStringUTF(env, cVal)
	var zeroString C.jstring
	if jstr == zeroString {
		return zeroString, fmt.Errorf("cannot allocate Java string: %s", value)
	}
	if err := jniException(env, "NewStringUTF"); err != nil {
		return zeroString, err
	}
	return jstr, nil
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
