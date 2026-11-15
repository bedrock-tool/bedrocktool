package android

import (
	"errors"
	"os"
	"path/filepath"

	"git.wow.st/gmp/jni"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
)

// setupUpdates lets the updater install APKs, and removes the one the last
// update installed from (it can't go while the installer reads it).
func setupUpdates() {
	updater.InstallAPK = installAPK
	_ = os.RemoveAll(utils.PathData("update"))
}

// installAPK opens Android's installer on a downloaded update. The user
// confirms there; Android then closes the app and installs the new one.
func installAPK(path string) error {
	uri, err := worldUri(filepath.Clean(path))
	if err != nil {
		return err
	}
	var opened bool
	err = withHelper(func(env jni.Env, cls jni.Class, ctx jni.Object) error {
		m := jni.GetStaticMethodID(env, cls, "installApk", "(Landroid/content/Context;Ljava/lang/String;)Z")
		var err error
		opened, err = jni.CallStaticBooleanMethod(env, cls, m, jni.Value(ctx), jstr(env, uri))
		return err
	})
	if err != nil {
		return err
	}
	if !opened {
		// Android's "install unknown apps" setting is open instead
		return errors.New(T("android_update_allow_install", nil))
	}
	return nil
}
