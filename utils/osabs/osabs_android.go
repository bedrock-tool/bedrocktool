package osabs

import (
	"fmt"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

func implInit() error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		// classes
		ctxClass := jni.FindClass(env, "android/content/Context")
		if ctxClass == 0 {
			return fmt.Errorf("failed to find Context class")
		}
		fileClass := jni.FindClass(env, "java/io/File")
		if fileClass == 0 {
			return fmt.Errorf("failed to find File class")
		}

		appCtx := jni.Object(app.AppContext())
		if appCtx == 0 {
			return fmt.Errorf("failed to get AppContext")
		}

		// methods
		getCacheDirMethod := jni.GetMethodID(env, ctxClass, "getCacheDir", "()Ljava/io/File;")
		if getCacheDirMethod == nil {
			return fmt.Errorf("failed to find getCacheDir")
		}

		getExternalFilesDirMethod := jni.GetMethodID(env, ctxClass, "getExternalFilesDir", "(Ljava/lang/String;)Ljava/io/File;")
		if getExternalFilesDirMethod == nil {
			return fmt.Errorf("failed to find getExternalFilesDir")
		}

		getAbsolutePathMethod := jni.GetMethodID(env, fileClass, "getAbsolutePath", "()Ljava/lang/String;")
		if getAbsolutePathMethod == nil {
			return fmt.Errorf("failed to find getAbsolutePath")
		}

		fmt.Printf("calling getCacheDir\n")
		cacheDirFileObj, err := jni.CallObjectMethod(env, appCtx, getCacheDirMethod)
		if err != nil {
			return fmt.Errorf("failed to call getCacheDir: %w", err)
		}

		{ // cache dir
			fmt.Printf("calling getAbsolutePath\n")
			pathStrObj, err := jni.CallObjectMethod(env, cacheDirFileObj, getAbsolutePathMethod)
			if err != nil {
				return fmt.Errorf("failed to call getAbsolutePath: %w", err)
			}
			cacheDir = jni.GoString(env, jni.String(pathStrObj))
		}

		{ // data dir
			fmt.Printf("calling getExternalFilesDir\n")
			filesDirFileObj, err := jni.CallObjectMethod(env, appCtx, getExternalFilesDirMethod, 0)
			if err != nil {
				return fmt.Errorf("failed to call getExternalFilesDir: %w", err)
			}

			fmt.Printf("calling getAbsolutePath\n")
			pathStrObj, err := jni.CallObjectMethod(env, filesDirFileObj, getAbsolutePathMethod)
			if err != nil {
				return fmt.Errorf("failed to call getAbsolutePath: %w", err)
			}
			dataDir = jni.GoString(env, jni.String(pathStrObj))
		}

		return nil
	})
}
