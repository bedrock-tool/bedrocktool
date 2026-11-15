package com.bedrocktool.android;

import android.app.Activity;
import android.app.Fragment;
import android.app.FragmentManager;
import android.content.ContentResolver;
import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.os.Handler;
import android.os.Looper;
import android.provider.MediaStore;
import android.view.View;
import android.widget.Toast;

import java.io.File;
import java.io.FileInputStream;
import java.io.InputStream;
import java.io.OutputStream;

// Android calls Bedrocktool can't make from Go alone. Built into
// bedrocktool_android.jar by build_jar.sh; gogio puts that jar in the APK.
public final class BedrocktoolAndroid {
    private BedrocktoolAndroid() {}

    // copyToDownloads copies the file at path into Downloads/<folder> through
    // MediaStore (no storage permission needed) and returns its content:// uri.
    // Needs Android 10 (API 29) or newer.
    public static String copyToDownloads(Context ctx, String path, String name, String folder) throws Exception {
        if (Build.VERSION.SDK_INT < 29) {
            throw new UnsupportedOperationException("needs Android 10 or newer");
        }
        ContentResolver cr = ctx.getContentResolver();
        ContentValues v = new ContentValues();
        v.put(MediaStore.MediaColumns.DISPLAY_NAME, name);
        v.put(MediaStore.MediaColumns.MIME_TYPE, "application/octet-stream");
        v.put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS + "/" + folder);
        v.put(MediaStore.MediaColumns.IS_PENDING, 1);
        Uri uri = cr.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, v);
        if (uri == null) {
            throw new IllegalStateException("could not create " + name + " in Downloads");
        }
        try (InputStream in = new FileInputStream(path); OutputStream out = cr.openOutputStream(uri)) {
            byte[] buf = new byte[1 << 16];
            int n;
            while ((n = in.read(buf)) > 0) {
                out.write(buf, 0, n);
            }
        } catch (Exception e) {
            cr.delete(uri, null, null);
            throw e;
        }
        v.clear();
        v.put(MediaStore.MediaColumns.IS_PENDING, 0);
        cr.update(uri, v, null, null);
        return uri.toString();
    }

    // exportFile opens the system's "save as" dialog for name and copies the
    // file at path to wherever the user picks, then says how it went in a
    // toast. view is the app's View (from Gio's ViewEvent): the dialog needs
    // its Activity. Gio's explorer can't do this for .mcworld: it asks
    // Android for the extension's MIME type, gets none, and the dialog
    // request without one matches no app.
    public static void exportFile(View view, final String path, final String name, final String done, final String failed) {
        final Activity act = (Activity) view.getContext();
        act.runOnUiThread(new Runnable() {
            public void run() {
                ExportFragment f = new ExportFragment();
                f.path = path;
                f.done = done;
                f.failed = failed;
                FragmentManager fm = act.getFragmentManager();
                fm.beginTransaction().add(f, "bedrocktool-export").commit();
                fm.executePendingTransactions();
                Intent i = new Intent(Intent.ACTION_CREATE_DOCUMENT);
                i.addCategory(Intent.CATEGORY_OPENABLE);
                i.setType("application/octet-stream");
                i.putExtra(Intent.EXTRA_TITLE, name);
                f.startActivityForResult(i, 1);
            }
        });
    }

    // ExportFragment only exists to get the dialog's result.
    public static class ExportFragment extends Fragment {
        String path, done, failed;

        @Override
        public void onActivityResult(int request, int result, Intent data) {
            final Context app = getActivity().getApplicationContext();
            getFragmentManager().beginTransaction().remove(this).commit();
            if (result != Activity.RESULT_OK || data == null || data.getData() == null || path == null) {
                return; // cancelled
            }
            final Uri uri = data.getData();
            new Thread(new Runnable() {
                public void run() {
                    String msg = done;
                    try (InputStream in = new FileInputStream(path);
                         OutputStream out = app.getContentResolver().openOutputStream(uri, "w")) {
                        byte[] buf = new byte[1 << 16];
                        int n;
                        while ((n = in.read(buf)) > 0) {
                            out.write(buf, 0, n);
                        }
                    } catch (Exception e) {
                        msg = failed + " " + e.getMessage();
                    }
                    final String m = msg;
                    new Handler(Looper.getMainLooper()).post(new Runnable() {
                        public void run() {
                            Toast.makeText(app, m, Toast.LENGTH_LONG).show();
                        }
                    });
                }
            }).start();
        }
    }

    // worldUri is the WorldProvider uri of a file in the app's files folder.
    public static String worldUri(Context ctx, String path) throws Exception {
        File root = ctx.getExternalFilesDir(null);
        if (root == null) {
            throw new IllegalStateException("no files folder");
        }
        String rootPath = root.getCanonicalPath() + File.separator;
        String filePath = new File(path).getCanonicalPath();
        if (!filePath.startsWith(rootPath)) {
            throw new IllegalArgumentException(path + " is not in " + rootPath);
        }
        Uri.Builder b = new Uri.Builder().scheme("content").authority(ctx.getPackageName() + ".worlds");
        for (String part : filePath.substring(rootPath.length()).split(File.separator)) {
            b.appendPath(part);
        }
        return b.build().toString();
    }

    // openInMinecraft hands a content:// uri to Minecraft, which imports the
    // .mcworld (or .mcpack) behind it. Minecraft decides what it got from the
    // uri's path, so the path has to end in .mcworld: a WorldProvider uri,
    // not a MediaStore one (those end in a number, and Minecraft ignores them).
    // CLEAR_TOP|SINGLE_TOP hand the file to a Minecraft that's already running
    // (onNewIntent) instead of starting a second one, which it can't survive.
    public static void openInMinecraft(Context ctx, String uri) throws Exception {
        Intent i = new Intent(Intent.ACTION_VIEW);
        i.setDataAndType(Uri.parse(uri), "application/octet-stream");
        i.setClassName("com.mojang.minecraftpe", "com.mojang.minecraftpe.MainActivity");
        i.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK
                | Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP);
        ctx.startActivity(i);
    }

    // startSession starts (or updates) SessionService, the foreground service
    // that keeps the app from being killed while a session runs.
    public static void startSession(Context ctx, String title, String text) {
        Intent i = new Intent(ctx, SessionService.class);
        i.putExtra(SessionService.EXTRA_TITLE, title);
        i.putExtra(SessionService.EXTRA_TEXT, text);
        if (Build.VERSION.SDK_INT >= 26) {
            ctx.startForegroundService(i);
        } else {
            ctx.startService(i);
        }
    }

    public static void stopSession(Context ctx) {
        ctx.stopService(new Intent(ctx, SessionService.class));
    }

    // askNotifications asks for the notification permission (Android 13 and
    // newer) so SessionService's notification shows. The service runs either
    // way; Android only asks the user a couple of times.
    public static void askNotifications(final View view) {
        if (Build.VERSION.SDK_INT < 33) {
            return;
        }
        final String perm = "android.permission.POST_NOTIFICATIONS";
        if (view.getContext().checkSelfPermission(perm) == PackageManager.PERMISSION_GRANTED) {
            return;
        }
        new Handler(Looper.getMainLooper()).post(new Runnable() {
            public void run() {
                ((Activity) view.getContext()).requestPermissions(new String[]{perm}, 2);
            }
        });
    }

    // installApk opens Android's installer on an update, served by
    // WorldProvider (uri from worldUri). Android 8+ only lets an app do that
    // once the user allows it to install apps: if they haven't, this opens
    // that setting instead and returns false.
    public static boolean installApk(Context ctx, String uri) {
        if (Build.VERSION.SDK_INT >= 26 && !ctx.getPackageManager().canRequestPackageInstalls()) {
            Intent s = new Intent(android.provider.Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                    Uri.parse("package:" + ctx.getPackageName()));
            s.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            ctx.startActivity(s);
            return false;
        }
        Intent i = new Intent(Intent.ACTION_VIEW);
        i.setDataAndType(Uri.parse(uri), "application/vnd.android.package-archive");
        i.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);
        ctx.startActivity(i);
        return true;
    }
}
