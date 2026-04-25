package ui

import (
	"io/fs"
	"net/http"
)

func Handler() http.Handler {
	sub, err := fs.Sub(FS, "dist")
	if err != nil {
		panic("ui: failed to sub dist: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if _, err := fs.Stat(sub, r.URL.Path[1:]); err != nil {

			r2 := *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, &r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
