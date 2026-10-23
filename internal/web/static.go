package web

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const notBuiltPage = `<!doctype html>
<meta charset="utf-8">
<title>Compose Assembler</title>
<body style="font-family:system-ui;max-width:40rem;margin:4rem auto;padding:0 1rem">
<h1>Frontend not built</h1>
<p>The API is running, but this binary contains no SPA assets.
Run <code>just frontend build</code> and rebuild the binary,
or use <code>just frontend dev</code> during development.</p>
</body>`

// spaHandler serves files from assets and falls back to index.html for every
// other path, so client-side routes survive a reload.
func spaHandler(assets fs.FS) http.Handler {
	files := http.FileServerFS(assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(assets, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") { // Vite output is content-hashed
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		serveIndex(w, r, assets)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	index, err := fs.ReadFile(assets, "index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if errors.Is(err, fs.ErrNotExist) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(notBuiltPage))
		return
	}
	if err != nil {
		http.Error(w, "cannot read index.html", http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(index)
}
