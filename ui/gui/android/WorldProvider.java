package com.bedrocktool.android;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;

import java.io.File;
import java.io.FileNotFoundException;
import java.io.IOException;

// WorldProvider lends saved worlds to Minecraft, read only. Its uris are
// content://<app id>.worlds/<path under the app's files folder>, so they end
// in the file's name (.mcworld), which is how Minecraft tells what it got.
// It isn't exported: an app can only open a uri it was handed with
// FLAG_GRANT_READ_URI_PERMISSION. Declared in manifest-application.xml.
public final class WorldProvider extends ContentProvider {
    @Override
    public boolean onCreate() {
        return true;
    }

    // fileFor maps a uri back to its file, which must be in the files folder.
    private File fileFor(Uri uri) throws FileNotFoundException {
        File root = getContext().getExternalFilesDir(null);
        String path = uri.getPath();
        if (root == null || path == null) {
            throw new FileNotFoundException(uri.toString());
        }
        try {
            File f = new File(root, path).getCanonicalFile();
            if (!f.getPath().startsWith(root.getCanonicalPath() + File.separator) || !f.isFile()) {
                throw new FileNotFoundException(uri.toString());
            }
            return f;
        } catch (IOException e) {
            throw new FileNotFoundException(uri + ": " + e.getMessage());
        }
    }

    @Override
    public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        if (!"r".equals(mode)) {
            throw new SecurityException("read only");
        }
        return ParcelFileDescriptor.open(fileFor(uri), ParcelFileDescriptor.MODE_READ_ONLY);
    }

    @Override
    public Cursor query(Uri uri, String[] projection, String selection, String[] args, String sort) {
        File f;
        try {
            f = fileFor(uri);
        } catch (FileNotFoundException e) {
            return null;
        }
        if (projection == null) {
            projection = new String[]{OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE};
        }
        MatrixCursor c = new MatrixCursor(projection, 1);
        Object[] row = new Object[projection.length];
        for (int i = 0; i < projection.length; i++) {
            if (OpenableColumns.DISPLAY_NAME.equals(projection[i])) {
                row[i] = f.getName();
            } else if (OpenableColumns.SIZE.equals(projection[i])) {
                row[i] = f.length();
            }
        }
        c.addRow(row);
        return c;
    }

    @Override
    public String getType(Uri uri) {
        String path = uri.getPath();
        if (path != null && path.endsWith(".apk")) {
            // the update the installer is given (update_android.go)
            return "application/vnd.android.package-archive";
        }
        return "application/octet-stream";
    }

    @Override
    public Uri insert(Uri uri, ContentValues values) {
        throw new UnsupportedOperationException("read only");
    }

    @Override
    public int delete(Uri uri, String selection, String[] args) {
        throw new UnsupportedOperationException("read only");
    }

    @Override
    public int update(Uri uri, ContentValues values, String selection, String[] args) {
        throw new UnsupportedOperationException("read only");
    }
}
