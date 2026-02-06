//go:build android

package ui

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
static jmethodID jni_GetMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetMethodID(env, clazz, name, sig);
}
static jmethodID jni_GetStaticMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetStaticMethodID(env, clazz, name, sig);
}
static jobject jni_CallObjectMethodNoArgs(JNIEnv *env, jobject obj, jmethodID methodID) {
	return (*env)->CallObjectMethod(env, obj, methodID);
}
static jobject jni_CallStaticObjectMethodNoArgs(JNIEnv *env, jclass clazz, jmethodID methodID) {
	return (*env)->CallStaticObjectMethod(env, clazz, methodID);
}
static const char* jni_GetStringUTFChars(JNIEnv *env, jstring str, jboolean *isCopy) {
	return (*env)->GetStringUTFChars(env, str, isCopy);
}
static void jni_ReleaseStringUTFChars(JNIEnv *env, jstring str, const char *chars) {
	(*env)->ReleaseStringUTFChars(env, str, chars);
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
	"strings"
	"unsafe"

	"gioui.org/app"

	"github.com/vitovt/wetterabhaengig/internal/gps"
	"github.com/vitovt/wetterabhaengig/internal/notify"
)

func (u *UI) handlePlatformEvent(event any) {
	viewEvent, ok := event.(app.AndroidViewEvent)
	if !ok {
		return
	}
	if binder, ok := u.gps.(gps.AndroidViewBinder); ok {
		binder.SetAndroidView(viewEvent.View)
	}
	if binder, ok := u.ntf.(notify.AndroidViewBinder); ok {
		binder.SetAndroidView(viewEvent.View)
	}
	if !u.platformLanguageInit && u.i18n != nil {
		if lang, err := androidDefaultLanguage(); err == nil && lang != "" {
			u.i18n.SetSystemLanguage(lang)
			u.refreshLanguagesFromBundle()
		}
		u.platformLanguageInit = true
	}
}

func androidDefaultLanguage() (string, error) {
	jvmPtr := app.JavaVM()
	if jvmPtr == 0 {
		return "", errors.New("android JVM is not initialized")
	}
	jvm := (*C.JavaVM)(unsafe.Pointer(jvmPtr))

	var env *C.JNIEnv
	res := C.jni_GetEnv(jvm, &env, C.JNI_VERSION_1_6)
	detach := false
	if res != C.JNI_OK {
		if res != C.JNI_EDETACHED {
			return "", fmt.Errorf("JNI GetEnv failed: %d", int(res))
		}
		if C.jni_AttachCurrentThread(jvm, &env, nil) != C.JNI_OK {
			return "", errors.New("JNI attach thread failed")
		}
		detach = true
	}
	if detach {
		defer C.jni_DetachCurrentThread(jvm)
	}

	localeClassName := C.CString("java/util/Locale")
	defer C.free(unsafe.Pointer(localeClassName))
	localeClass := C.jni_FindClass(env, localeClassName)
	var zeroClass C.jclass
	var zeroObj C.jobject
	if localeClass == zeroClass {
		return "", errors.New("cannot resolve java.util.Locale class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(localeClass))

	getDefaultMethod, err := getStaticMethodID(env, localeClass, "getDefault", "()Ljava/util/Locale;")
	if err != nil {
		return "", err
	}
	localeObj := C.jni_CallStaticObjectMethodNoArgs(env, localeClass, getDefaultMethod)
	if err := jniException(env, "Locale.getDefault"); err != nil {
		return "", err
	}
	if localeObj == zeroObj {
		return "", errors.New("default locale is unavailable")
	}
	defer C.jni_DeleteLocalRef(env, localeObj)

	getLanguageMethod, err := getMethodID(env, localeClass, "getLanguage", "()Ljava/lang/String;")
	if err != nil {
		return "", err
	}
	langStr := C.jni_CallObjectMethodNoArgs(env, localeObj, getLanguageMethod)
	if err := jniException(env, "Locale.getLanguage"); err != nil {
		return "", err
	}
	if langStr == zeroObj {
		return "", errors.New("locale language is unavailable")
	}
	defer C.jni_DeleteLocalRef(env, langStr)

	lang, err := goStringFromJString(env, C.jstring(langStr))
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(lang)), nil
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

func goStringFromJString(env *C.JNIEnv, str C.jstring) (string, error) {
	var isCopy C.jboolean
	chars := C.jni_GetStringUTFChars(env, str, &isCopy)
	if chars == nil {
		if err := jniException(env, "GetStringUTFChars"); err != nil {
			return "", err
		}
		return "", errors.New("cannot access Java string bytes")
	}
	defer C.jni_ReleaseStringUTFChars(env, str, chars)
	return C.GoString(chars), nil
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
