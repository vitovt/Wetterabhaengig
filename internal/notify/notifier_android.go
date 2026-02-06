//go:build android

package notify

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
static jfieldID jni_GetFieldID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetFieldID(env, clazz, name, sig);
}
static jfieldID jni_GetStaticFieldID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetStaticFieldID(env, clazz, name, sig);
}
static jint jni_GetIntField(JNIEnv *env, jobject obj, jfieldID fieldID) {
	return (*env)->GetIntField(env, obj, fieldID);
}
static jint jni_GetStaticIntField(JNIEnv *env, jclass clazz, jfieldID fieldID) {
	return (*env)->GetStaticIntField(env, clazz, fieldID);
}
static jint jni_CallIntMethodStr(JNIEnv *env, jobject obj, jmethodID methodID, jstring str) {
	return (*env)->CallIntMethod(env, obj, methodID, str);
}
static jint jni_CallIntMethod3Str(JNIEnv *env, jobject obj, jmethodID methodID, jstring a, jstring b, jstring c) {
	return (*env)->CallIntMethod(env, obj, methodID, a, b, c);
}
static jobject jni_CallObjectMethodStr(JNIEnv *env, jobject obj, jmethodID methodID, jstring str) {
	return (*env)->CallObjectMethod(env, obj, methodID, str);
}
static jobject jni_CallObjectMethodNoArgs(JNIEnv *env, jobject obj, jmethodID methodID) {
	return (*env)->CallObjectMethod(env, obj, methodID);
}
static jobject jni_CallObjectMethodObj(JNIEnv *env, jobject obj, jmethodID methodID, jobject arg) {
	return (*env)->CallObjectMethod(env, obj, methodID, arg);
}
static jobject jni_CallObjectMethodInt(JNIEnv *env, jobject obj, jmethodID methodID, jint arg) {
	return (*env)->CallObjectMethod(env, obj, methodID, arg);
}
static jobject jni_CallObjectMethodBool(JNIEnv *env, jobject obj, jmethodID methodID, jboolean arg) {
	return (*env)->CallObjectMethod(env, obj, methodID, arg);
}
static void jni_CallVoidMethodPermReq(JNIEnv *env, jobject obj, jmethodID methodID, jobjectArray perms, jint reqCode) {
	(*env)->CallVoidMethod(env, obj, methodID, perms, reqCode);
}
static void jni_CallVoidMethodObj(JNIEnv *env, jobject obj, jmethodID methodID, jobject arg) {
	(*env)->CallVoidMethod(env, obj, methodID, arg);
}
static void jni_CallVoidMethodIntObj(JNIEnv *env, jobject obj, jmethodID methodID, jint id, jobject arg) {
	(*env)->CallVoidMethod(env, obj, methodID, id, arg);
}
static jobject jni_NewObjectObj(JNIEnv *env, jclass clazz, jmethodID methodID, jobject arg) {
	return (*env)->NewObject(env, clazz, methodID, arg);
}
static jobject jni_NewObjectObjStr(JNIEnv *env, jclass clazz, jmethodID methodID, jobject arg1, jstring arg2) {
	return (*env)->NewObject(env, clazz, methodID, arg1, arg2);
}
static jobject jni_NewObjectStrStrInt(JNIEnv *env, jclass clazz, jmethodID methodID, jstring arg1, jstring arg2, jint arg3) {
	return (*env)->NewObject(env, clazz, methodID, arg1, arg2, arg3);
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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"gioui.org/app"
)

const (
	permissionGranted              = 0
	notificationPermissionRequest  = 4208
	notificationPermission         = "android.permission.POST_NOTIFICATIONS"
	notificationChannelID          = "wetterabhaengig_alerts"
	notificationChannelName        = "Wetterabhaengig Alerts"
	notificationChannelImportance  = 3
	minNotificationPermissionAPI   = 33
	minNotificationChannelAPI      = 26
	defaultNotificationAndroidIcon = 17301659
	notificationSmallIconName      = "ic_stat_wetterabhaengig"
)

type androidNotifier struct {
	mu     sync.RWMutex
	view   uintptr
	nextID atomic.Int32
}

func newNotifier() Notifier {
	return &androidNotifier{}
}

func (n *androidNotifier) SetAndroidView(view uintptr) {
	n.mu.Lock()
	n.view = view
	n.mu.Unlock()
}

func (n *androidNotifier) Send(title, body string) error {
	return withJNIEnv(func(env *C.JNIEnv, appCtx C.jobject) error {
		var zeroObj C.jobject
		activity, activityErr := n.activityFromView(env)
		hasActivity := activityErr == nil && activity != zeroObj
		if hasActivity {
			defer C.jni_DeleteLocalRef(env, activity)
		}

		sdkInt, err := androidSDKInt(env)
		if err != nil {
			return err
		}
		if sdkInt >= minNotificationPermissionAPI {
			permissionCtx := appCtx
			if hasActivity {
				permissionCtx = activity
			}
			granted, err := checkPermission(env, permissionCtx, notificationPermission)
			if err != nil {
				return err
			}
			if !granted {
				if hasActivity {
					if err := requestPermission(env, activity, notificationPermission, notificationPermissionRequest); err != nil {
						return fmt.Errorf("cannot request notification permission dialog: %w", err)
					}
					return errors.New("notification permission is not granted yet; allow it and press Test notification again (if no dialog appears, rebuild APK with POST_NOTIFICATIONS in manifest)")
				}
				return errors.New("notification permission is not granted for background delivery; open the app and allow notifications")
			}
		}

		manager, err := getSystemService(env, appCtx, "notification")
		if err != nil {
			return err
		}
		defer C.jni_DeleteLocalRef(env, manager)

		if sdkInt >= minNotificationChannelAPI {
			if err := ensureNotificationChannel(env, manager); err != nil {
				return err
			}
		}

		iconID, err := resolveSmallIcon(env, appCtx)
		if err != nil {
			iconID = defaultNotificationAndroidIcon
		}
		notificationObj, err := buildNotification(env, appCtx, sdkInt, title, body, iconID)
		if err != nil {
			return err
		}
		defer C.jni_DeleteLocalRef(env, notificationObj)

		id := C.jint(n.nextID.Add(1))
		return postNotification(env, manager, id, notificationObj)
	})
}

func (n *androidNotifier) activityFromView(env *C.JNIEnv) (C.jobject, error) {
	n.mu.RLock()
	viewPtr := n.view
	n.mu.RUnlock()

	var zeroObj C.jobject
	if viewPtr == 0 {
		return zeroObj, errors.New("android view is not ready yet")
	}
	viewObj := C.jobject(unsafe.Pointer(viewPtr))

	viewClass := C.jni_GetObjectClass(env, viewObj)
	var zeroClass C.jclass
	if viewClass == zeroClass {
		return zeroObj, errors.New("cannot resolve Android View class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(viewClass))

	getContextMethod, err := getMethodID(env, viewClass, "getContext", "()Landroid/content/Context;")
	if err != nil {
		return zeroObj, err
	}
	ctx := C.jni_CallObjectMethodNoArgs(env, viewObj, getContextMethod)
	if err := jniException(env, "View.getContext"); err != nil {
		return zeroObj, err
	}
	if ctx == zeroObj {
		return zeroObj, errors.New("android view context is unavailable")
	}

	activityClassName := C.CString("android/app/Activity")
	defer C.free(unsafe.Pointer(activityClassName))
	activityClass := C.jni_FindClass(env, activityClassName)
	if activityClass == zeroClass {
		C.jni_DeleteLocalRef(env, ctx)
		return zeroObj, errors.New("cannot resolve android.app.Activity class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(activityClass))

	if C.jni_IsInstanceOf(env, ctx, activityClass) != C.JNI_TRUE {
		C.jni_DeleteLocalRef(env, ctx)
		return zeroObj, errors.New("android view context is not Activity")
	}
	return ctx, nil
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

func androidSDKInt(env *C.JNIEnv) (int, error) {
	className := C.CString("android/os/Build$VERSION")
	defer C.free(unsafe.Pointer(className))
	versionClass := C.jni_FindClass(env, className)
	var zeroClass C.jclass
	if versionClass == zeroClass {
		return 0, errors.New("cannot resolve android.os.Build$VERSION class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(versionClass))

	field, err := getStaticFieldID(env, versionClass, "SDK_INT", "I")
	if err != nil {
		return 0, err
	}
	sdk := C.jni_GetStaticIntField(env, versionClass, field)
	if err := jniException(env, "GetStaticIntField:SDK_INT"); err != nil {
		return 0, err
	}
	return int(sdk), nil
}

func checkPermission(env *C.JNIEnv, ctx C.jobject, permission string) (bool, error) {
	ctxClass := C.jni_GetObjectClass(env, ctx)
	var zeroClass C.jclass
	if ctxClass == zeroClass {
		return false, errors.New("cannot get Android context class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(ctxClass))

	method, err := getMethodID(env, ctxClass, "checkSelfPermission", "(Ljava/lang/String;)I")
	if err != nil {
		return false, err
	}

	perm, err := newJavaString(env, permission)
	if err != nil {
		return false, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(perm))

	res := C.jni_CallIntMethodStr(env, ctx, method, perm)
	if err := jniException(env, "checkSelfPermission"); err != nil {
		return false, err
	}
	return int(res) == permissionGranted, nil
}

func requestPermission(env *C.JNIEnv, activity C.jobject, permission string, requestCode int) error {
	activityClassName := C.CString("android/app/Activity")
	defer C.free(unsafe.Pointer(activityClassName))
	activityClass := C.jni_FindClass(env, activityClassName)
	var zeroClass C.jclass
	if activityClass == zeroClass {
		return errors.New("cannot resolve android.app.Activity class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(activityClass))
	if C.jni_IsInstanceOf(env, activity, activityClass) != C.JNI_TRUE {
		return errors.New("request target is not an Activity; cannot show permission dialog")
	}

	ctxClass := C.jni_GetObjectClass(env, activity)
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

	perm, err := newJavaString(env, permission)
	if err != nil {
		return err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(perm))

	C.jni_SetObjectArrayElement(env, perms, 0, C.jobject(perm))
	if err := jniException(env, "SetObjectArrayElement"); err != nil {
		return err
	}

	C.jni_CallVoidMethodPermReq(env, activity, requestMethod, perms, C.jint(requestCode))
	return jniException(env, "requestPermissions")
}

func getSystemService(env *C.JNIEnv, appCtx C.jobject, serviceName string) (C.jobject, error) {
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

	service, err := newJavaString(env, serviceName)
	if err != nil {
		return zeroObj, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(service))

	manager := C.jni_CallObjectMethodStr(env, appCtx, getSystemServiceMethod, service)
	if err := jniException(env, "getSystemService"); err != nil {
		return zeroObj, err
	}
	if manager == zeroObj {
		return zeroObj, fmt.Errorf("%s service is unavailable", serviceName)
	}
	return manager, nil
}

func ensureNotificationChannel(env *C.JNIEnv, manager C.jobject) error {
	channelClassName := C.CString("android/app/NotificationChannel")
	defer C.free(unsafe.Pointer(channelClassName))
	channelClass := C.jni_FindClass(env, channelClassName)
	var zeroClass C.jclass
	var zeroObj C.jobject
	if channelClass == zeroClass {
		return errors.New("cannot resolve NotificationChannel class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(channelClass))

	ctor, err := getMethodID(env, channelClass, "<init>", "(Ljava/lang/String;Ljava/lang/CharSequence;I)V")
	if err != nil {
		return err
	}

	channelID, err := newJavaString(env, notificationChannelID)
	if err != nil {
		return err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(channelID))
	channelName, err := newJavaString(env, notificationChannelName)
	if err != nil {
		return err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(channelName))

	channel := C.jni_NewObjectStrStrInt(env, channelClass, ctor, channelID, channelName, C.jint(notificationChannelImportance))
	if channel == zeroObj {
		return errors.New("cannot allocate NotificationChannel")
	}
	defer C.jni_DeleteLocalRef(env, channel)
	if err := jniException(env, "NewObject:NotificationChannel"); err != nil {
		return err
	}

	managerClass := C.jni_GetObjectClass(env, manager)
	if managerClass == zeroClass {
		return errors.New("cannot get NotificationManager class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(managerClass))

	createMethod, err := getMethodID(env, managerClass, "createNotificationChannel", "(Landroid/app/NotificationChannel;)V")
	if err != nil {
		return err
	}
	C.jni_CallVoidMethodObj(env, manager, createMethod, channel)
	return jniException(env, "createNotificationChannel")
}

func resolveSmallIcon(env *C.JNIEnv, appCtx C.jobject) (int, error) {
	iconID, err := resolveResourceIdentifier(env, appCtx, notificationSmallIconName, "drawable")
	if err == nil && iconID != 0 {
		return iconID, nil
	}
	if err == nil {
		if launcherID, lookupErr := resolveResourceIdentifier(env, appCtx, "ic_launcher", "mipmap"); lookupErr == nil && launcherID != 0 {
			return launcherID, nil
		}
	}

	ctxClass := C.jni_GetObjectClass(env, appCtx)
	var zeroClass C.jclass
	var zeroObj C.jobject
	if ctxClass == zeroClass {
		return 0, errors.New("cannot get context class for icon")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(ctxClass))

	getAppInfoMethod, err := getMethodID(env, ctxClass, "getApplicationInfo", "()Landroid/content/pm/ApplicationInfo;")
	if err != nil {
		return 0, err
	}
	appInfo := C.jni_CallObjectMethodNoArgs(env, appCtx, getAppInfoMethod)
	if err := jniException(env, "getApplicationInfo"); err != nil {
		return 0, err
	}
	if appInfo == zeroObj {
		return 0, errors.New("application info is unavailable")
	}
	defer C.jni_DeleteLocalRef(env, appInfo)

	appInfoClass := C.jni_GetObjectClass(env, appInfo)
	if appInfoClass == zeroClass {
		return 0, errors.New("cannot get ApplicationInfo class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(appInfoClass))

	iconField, err := getFieldID(env, appInfoClass, "icon", "I")
	if err != nil {
		return 0, err
	}
	icon := C.jni_GetIntField(env, appInfo, iconField)
	if err := jniException(env, "GetIntField:ApplicationInfo.icon"); err != nil {
		return 0, err
	}
	if icon != 0 {
		return int(icon), nil
	}

	return defaultNotificationAndroidIcon, nil
}

func resolveResourceIdentifier(env *C.JNIEnv, appCtx C.jobject, resourceName, resourceType string) (int, error) {
	ctxClass := C.jni_GetObjectClass(env, appCtx)
	var zeroClass C.jclass
	var zeroObj C.jobject
	if ctxClass == zeroClass {
		return 0, errors.New("cannot get context class for resource lookup")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(ctxClass))

	getResourcesMethod, err := getMethodID(env, ctxClass, "getResources", "()Landroid/content/res/Resources;")
	if err != nil {
		return 0, err
	}
	getPackageNameMethod, err := getMethodID(env, ctxClass, "getPackageName", "()Ljava/lang/String;")
	if err != nil {
		return 0, err
	}

	resourcesObj := C.jni_CallObjectMethodNoArgs(env, appCtx, getResourcesMethod)
	if err := jniException(env, "getResources"); err != nil {
		return 0, err
	}
	if resourcesObj == zeroObj {
		return 0, errors.New("resources object is unavailable")
	}
	defer C.jni_DeleteLocalRef(env, resourcesObj)

	packageNameObj := C.jni_CallObjectMethodNoArgs(env, appCtx, getPackageNameMethod)
	if err := jniException(env, "getPackageName"); err != nil {
		return 0, err
	}
	if packageNameObj == zeroObj {
		return 0, errors.New("package name is unavailable")
	}
	defer C.jni_DeleteLocalRef(env, packageNameObj)

	resourcesClass := C.jni_GetObjectClass(env, resourcesObj)
	if resourcesClass == zeroClass {
		return 0, errors.New("cannot get Resources class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(resourcesClass))

	getIdentifierMethod, err := getMethodID(env, resourcesClass, "getIdentifier", "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)I")
	if err != nil {
		return 0, err
	}

	resName, err := newJavaString(env, resourceName)
	if err != nil {
		return 0, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(resName))
	resType, err := newJavaString(env, resourceType)
	if err != nil {
		return 0, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(resType))

	resID := C.jni_CallIntMethod3Str(env, resourcesObj, getIdentifierMethod, resName, resType, C.jstring(packageNameObj))
	if err := jniException(env, "Resources.getIdentifier"); err != nil {
		return 0, err
	}
	return int(resID), nil
}

func buildNotification(env *C.JNIEnv, appCtx C.jobject, sdkInt int, title, body string, smallIcon int) (C.jobject, error) {
	builderClassName := C.CString("android/app/Notification$Builder")
	defer C.free(unsafe.Pointer(builderClassName))
	builderClass := C.jni_FindClass(env, builderClassName)
	var zeroClass C.jclass
	var zeroObj C.jobject
	if builderClass == zeroClass {
		return zeroObj, errors.New("cannot resolve Notification.Builder class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(builderClass))

	var builder C.jobject
	if sdkInt >= minNotificationChannelAPI {
		channelID, err := newJavaString(env, notificationChannelID)
		if err != nil {
			return zeroObj, err
		}
		defer C.jni_DeleteLocalRef(env, C.jobject(channelID))

		ctor, err := getMethodID(env, builderClass, "<init>", "(Landroid/content/Context;Ljava/lang/String;)V")
		if err != nil {
			return zeroObj, err
		}
		builder = C.jni_NewObjectObjStr(env, builderClass, ctor, appCtx, channelID)
		if builder == zeroObj {
			return zeroObj, errors.New("cannot allocate Notification.Builder")
		}
		if err := jniException(env, "NewObject:Notification.Builder(context,channel)"); err != nil {
			return zeroObj, err
		}
	} else {
		ctor, err := getMethodID(env, builderClass, "<init>", "(Landroid/content/Context;)V")
		if err != nil {
			return zeroObj, err
		}
		builder = C.jni_NewObjectObj(env, builderClass, ctor, appCtx)
		if builder == zeroObj {
			return zeroObj, errors.New("cannot allocate Notification.Builder")
		}
		if err := jniException(env, "NewObject:Notification.Builder(context)"); err != nil {
			return zeroObj, err
		}
	}
	defer C.jni_DeleteLocalRef(env, builder)

	setContentTitleMethod, err := getMethodID(env, builderClass, "setContentTitle", "(Ljava/lang/CharSequence;)Landroid/app/Notification$Builder;")
	if err != nil {
		return zeroObj, err
	}
	setContentTextMethod, err := getMethodID(env, builderClass, "setContentText", "(Ljava/lang/CharSequence;)Landroid/app/Notification$Builder;")
	if err != nil {
		return zeroObj, err
	}
	setSmallIconMethod, err := getMethodID(env, builderClass, "setSmallIcon", "(I)Landroid/app/Notification$Builder;")
	if err != nil {
		return zeroObj, err
	}
	setAutoCancelMethod, err := getMethodID(env, builderClass, "setAutoCancel", "(Z)Landroid/app/Notification$Builder;")
	if err != nil {
		return zeroObj, err
	}
	buildMethod, err := getMethodID(env, builderClass, "build", "()Landroid/app/Notification;")
	if err != nil {
		return zeroObj, err
	}

	titleStr, err := newJavaString(env, title)
	if err != nil {
		return zeroObj, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(titleStr))
	bodyStr, err := newJavaString(env, body)
	if err != nil {
		return zeroObj, err
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(bodyStr))

	if err := callBuilderObjectMethodObj(env, builder, setContentTitleMethod, C.jobject(titleStr), "setContentTitle"); err != nil {
		return zeroObj, err
	}
	if err := callBuilderObjectMethodObj(env, builder, setContentTextMethod, C.jobject(bodyStr), "setContentText"); err != nil {
		return zeroObj, err
	}
	if err := callBuilderObjectMethodInt(env, builder, setSmallIconMethod, C.jint(smallIcon), "setSmallIcon"); err != nil {
		return zeroObj, err
	}
	if err := callBuilderObjectMethodBool(env, builder, setAutoCancelMethod, C.JNI_TRUE, "setAutoCancel"); err != nil {
		return zeroObj, err
	}

	notification := C.jni_CallObjectMethodNoArgs(env, builder, buildMethod)
	if err := jniException(env, "Notification.Builder.build"); err != nil {
		return zeroObj, err
	}
	if notification == zeroObj {
		return zeroObj, errors.New("notification build returned nil")
	}
	return notification, nil
}

func callBuilderObjectMethodObj(env *C.JNIEnv, builder C.jobject, method C.jmethodID, arg C.jobject, where string) error {
	result := C.jni_CallObjectMethodObj(env, builder, method, arg)
	if err := jniException(env, where); err != nil {
		return err
	}
	var zeroObj C.jobject
	if result != zeroObj {
		C.jni_DeleteLocalRef(env, result)
	}
	return nil
}

func callBuilderObjectMethodInt(env *C.JNIEnv, builder C.jobject, method C.jmethodID, arg C.jint, where string) error {
	result := C.jni_CallObjectMethodInt(env, builder, method, arg)
	if err := jniException(env, where); err != nil {
		return err
	}
	var zeroObj C.jobject
	if result != zeroObj {
		C.jni_DeleteLocalRef(env, result)
	}
	return nil
}

func callBuilderObjectMethodBool(env *C.JNIEnv, builder C.jobject, method C.jmethodID, arg C.jboolean, where string) error {
	result := C.jni_CallObjectMethodBool(env, builder, method, arg)
	if err := jniException(env, where); err != nil {
		return err
	}
	var zeroObj C.jobject
	if result != zeroObj {
		C.jni_DeleteLocalRef(env, result)
	}
	return nil
}

func postNotification(env *C.JNIEnv, manager C.jobject, id C.jint, notification C.jobject) error {
	managerClass := C.jni_GetObjectClass(env, manager)
	var zeroClass C.jclass
	if managerClass == zeroClass {
		return errors.New("cannot get NotificationManager class")
	}
	defer C.jni_DeleteLocalRef(env, C.jobject(managerClass))

	notifyMethod, err := getMethodID(env, managerClass, "notify", "(ILandroid/app/Notification;)V")
	if err != nil {
		return err
	}
	C.jni_CallVoidMethodIntObj(env, manager, notifyMethod, id, notification)
	return jniException(env, "NotificationManager.notify")
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

func getFieldID(env *C.JNIEnv, class C.jclass, name, sig string) (C.jfieldID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSig := C.CString(sig)
	defer C.free(unsafe.Pointer(cSig))

	field := C.jni_GetFieldID(env, class, cName, cSig)
	var zeroField C.jfieldID
	if field == zeroField {
		return zeroField, fmt.Errorf("android field not found: %s %s", name, sig)
	}
	if err := jniException(env, "GetFieldID:"+name); err != nil {
		return zeroField, err
	}
	return field, nil
}

func getStaticFieldID(env *C.JNIEnv, class C.jclass, name, sig string) (C.jfieldID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSig := C.CString(sig)
	defer C.free(unsafe.Pointer(cSig))

	field := C.jni_GetStaticFieldID(env, class, cName, cSig)
	var zeroField C.jfieldID
	if field == zeroField {
		return zeroField, fmt.Errorf("android static field not found: %s %s", name, sig)
	}
	if err := jniException(env, "GetStaticFieldID:"+name); err != nil {
		return zeroField, err
	}
	return field, nil
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
