package android

import (
	"errors"
	"sync"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

// helperClass is BedrocktoolAndroid.java, built into bedrocktool_android.jar.
const helperClass = "com/bedrocktool/android/BedrocktoolAndroid"

// withHelper runs f with the helper class loaded and the app's Context.
// The class has to come from the app's class loader: a Go thread's JNI
// FindClass only sees Android's own classes.
func withHelper(f func(env jni.Env, cls jni.Class, ctx jni.Object) error) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		ctx := jni.Object(app.AppContext())
		cls, err := jni.LoadClass(env, jni.ClassLoaderFor(env, ctx), helperClass)
		if err != nil {
			return err
		}
		return f(env, cls, ctx)
	})
}

func jstr(env jni.Env, s string) jni.Value {
	return jni.Value(jni.JavaString(env, s))
}

// copyToDownloads copies the file at path to Downloads/<folder>/<name> and
// returns the content:// uri of the copy.
func copyToDownloads(path, name, folder string) (uri string, err error) {
	err = withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
		m := jni.GetStaticMethodID(env, cls, "copyToDownloads",
			"(Landroid/content/Context;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;")
		res, err := jni.CallStaticObjectMethod(env, cls, m, jni.Value(ctx), jstr(env, path), jstr(env, name), jstr(env, folder))
		if err != nil {
			return err
		}
		uri = jni.GoString(env, jni.String(res))
		return nil
	})
	return uri, err
}

var (
	viewMu sync.Mutex
	// view is the android.view.View of the window, from Gio's ViewEvent;
	// dialogs need its Activity. 0 while there is none.
	view uintptr
)

// SetView keeps the window's View from Gio's latest AndroidViewEvent.
func SetView(v uintptr) {
	viewMu.Lock()
	view = v
	viewMu.Unlock()
}

var errNoView = errors.New("the app's window isn't ready yet")

// exportFile opens the "save as" dialog; Java copies the file and shows a
// toast with done, or failed and the error.
func exportFile(path, name, done, failed string) error {
	viewMu.Lock()
	defer viewMu.Unlock()
	if view == 0 {
		return errNoView
	}
	return withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
		m := jni.GetStaticMethodID(env, cls, "exportFile",
			"(Landroid/view/View;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)V")
		return jni.CallStaticVoidMethod(env, cls, m, jni.Value(view), jstr(env, path), jstr(env, name), jstr(env, done), jstr(env, failed))
	})
}

// worldUri is the WorldProvider content:// uri of a file in the app's files
// folder; it ends in the file's name, which Minecraft needs.
func worldUri(path string) (uri string, err error) {
	err = withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
		m := jni.GetStaticMethodID(env, cls, "worldUri", "(Landroid/content/Context;Ljava/lang/String;)Ljava/lang/String;")
		res, err := jni.CallStaticObjectMethod(env, cls, m, jni.Value(ctx), jstr(env, path))
		if err != nil {
			return err
		}
		uri = jni.GoString(env, jni.String(res))
		return nil
	})
	return uri, err
}

// openInMinecraft has Minecraft import the world behind a content:// uri.
func openInMinecraft(uri string) error {
	return withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
		m := jni.GetStaticMethodID(env, cls, "openInMinecraft", "(Landroid/content/Context;Ljava/lang/String;)V")
		return jni.CallStaticVoidMethod(env, cls, m, jni.Value(ctx), jstr(env, uri))
	})
}
