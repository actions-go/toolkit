package cacheparity

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// The test server stands in for api.github.com. The repository name selects
// what /repos/<owner>/<repo>/git/trees/<branch> answers and, for the
// repositories that have a manifest, what the blob it points at contains.

// bom is a byte order mark.
const bom = "\xef\xbb\xbf"

// manifestBody is a manifest. releaseURL tells what the server saw so that the
// headers and the branch of both requests are compared.
func manifestBody(releaseURL string) string {
	return fmt.Sprintf(`[{"version":"1.2.3","stable":true,"release_url":%q,"files":[`+
		`{"filename":"t-linux-x64.tar.gz","platform":"linux","arch":"x64","download_url":"https://example.invalid/t-linux-x64.tar.gz"},`+
		`{"filename":"t-darwin-arm64.tar.gz","platform":"darwin","arch":"arm64","download_url":"https://example.invalid/t-darwin-arm64.tar.gz"},`+
		`{"filename":"t-win32-x64.zip","platform":"win32","arch":"x64","download_url":"https://example.invalid/t-win32-x64.zip"}]}]`, releaseURL)
}

func seen(r *http.Request) string {
	return fmt.Sprintf("%s|%s|%s", r.Header.Get("Accept"), r.Header.Get("Authorization"), r.Header.Get("User-Agent"))
}

func addManifestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		// /repos/<owner>/<repo>/git/trees/<branch>
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/repos/"), "/")
		if len(parts) < 5 {
			http.NotFound(w, r)
			return
		}
		repo, branch := parts[1], parts[4]
		base := "http://" + r.Host
		blob := func(kind string) string {
			return base + "/blob/" + kind + "?branch=" + url.QueryEscape(branch) + "&tree=" + url.QueryEscape(seen(r))
		}
		tree := func(items ...string) {
			_, _ = fmt.Fprintf(w, `{"sha":"abc","tree":[%s]}`, strings.Join(items, ","))
		}
		item := func(path, u string) string { return fmt.Sprintf(`{"path":%q,"url":%q}`, path, u) }
		switch repo {
		case "bom", "badjson", "empty", "null", "object", "blob404", "ok":
			tree(item("README.md", base+"/blob/other"), item("versions-manifest.json", blob(repo)))
		case "duplicate":
			tree(item("versions-manifest.json", blob("ok")), item("versions-manifest.json", blob("badjson")))
		case "blobredirect":
			tree(item("versions-manifest.json", base+"/blob-redirect?branch="+url.QueryEscape(branch)+"&tree="+url.QueryEscape(seen(r))))
		case "nomanifest":
			tree(item("README.md", base+"/blob/other"))
		case "emptytree":
			tree()
		case "notree":
			_, _ = w.Write([]byte(`{"foo":1}`))
		case "notjson":
			_, _ = w.Write([]byte("not json"))
		case "emptybody":
		case "jsonnull":
			_, _ = w.Write([]byte("null"))
		case "notfound":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		case "unauthorized":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
		case "forbidden":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
		case "servererror":
			w.WriteHeader(http.StatusInternalServerError)
		case "texterror":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad gateway"))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/blob/", func(w http.ResponseWriter, r *http.Request) {
		kind := strings.TrimPrefix(r.URL.Path, "/blob/")
		release := fmt.Sprintf("branch=%s tree=%s blob=%s", r.URL.Query().Get("branch"), r.URL.Query().Get("tree"), seen(r))
		switch kind {
		case "ok":
			_, _ = w.Write([]byte(manifestBody(release)))
		case "bom":
			_, _ = w.Write([]byte(bom + manifestBody(release)))
		case "badjson":
			_, _ = w.Write([]byte("{not json"))
		case "null":
			_, _ = w.Write([]byte("null"))
		case "object":
			_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
		case "empty":
		case "blob404":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/blob-redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+r.Host+"/blob/ok?"+r.URL.RawQuery, http.StatusFound)
	})
}
