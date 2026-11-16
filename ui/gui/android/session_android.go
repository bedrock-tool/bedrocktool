package android

import (
	"git.wow.st/gmp/jni"
	"github.com/sirupsen/logrus"
)

// SessionRunning starts SessionService when a subcommand (name) starts and
// stops it when it ends. The service is a foreground service: while it
// runs, Android shows a notification and leaves the app running in the
// background, where it is while the user plays in Minecraft.
func SessionRunning(name string, running bool) {
	var err error
	if running {
		askNotifications()
		err = withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
			m := jni.GetStaticMethodID(env, cls, "startSession", "(Landroid/content/Context;Ljava/lang/String;Ljava/lang/String;)V")
			return jni.CallStaticVoidMethod(env, cls, m, jni.Value(ctx),
				jstr(env, T("android_session_title", nil)), jstr(env, sessionText(name)))
		})
	} else {
		err = withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
			m := jni.GetStaticMethodID(env, cls, "stopSession", "(Landroid/content/Context;)V")
			return jni.CallStaticVoidMethod(env, cls, m, jni.Value(ctx))
		})
	}
	if err != nil {
		logrus.Warnf("background service: %s", err)
	}
}

func sessionText(name string) string {
	switch name {
	case "worlds":
		return T("android_session_worlds", nil)
	case "skins":
		return T("android_session_skins", nil)
	case "packs":
		return T("android_session_packs", nil)
	}
	return T("android_session_text", nil)
}

// askNotifications asks once per run for the notification permission
// (Android 13 and newer), which the service's notification needs.
func askNotifications() {
	viewMu.Lock()
	defer viewMu.Unlock()
	if view == 0 || askedNotifications {
		return
	}
	askedNotifications = true
	err := withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
		m := jni.GetStaticMethodID(env, cls, "askNotifications", "(Landroid/view/View;)V")
		return jni.CallStaticVoidMethod(env, cls, m, jni.Value(view))
	})
	if err != nil {
		logrus.Warnf("notification permission: %s", err)
	}
}

var askedNotifications bool
