package ginx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestDirHandlerWithSPA(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest"), []byte("manifest content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/assets/not-exists.css", wantStatus: http.StatusOK, wantBody: "<html>spa</html>"},
		{path: "/images/not-exists.png", wantStatus: http.StatusOK, wantBody: "<html>spa</html>"},
		{path: "/fonts/not-exists", wantStatus: http.StatusOK, wantBody: "<html>spa</html>"},
		{path: "/manifest", wantStatus: http.StatusOK, wantBody: "manifest content"},
		{path: "/support", wantStatus: http.StatusOK, wantBody: "<html>spa</html>"},
	}
	handler := DirHandler(http.Dir(root), DirOptions{
		ShowList:  false,
		SPA:       true,
		IndexName: "index.html",
	})

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)

		handler(ctx)

		if rec.Code != tt.wantStatus {
			t.Fatalf("%s status=%d want %d; body=%q", tt.path, rec.Code, tt.wantStatus, rec.Body.String())
		}
		if tt.wantBody != "" && strings.TrimSpace(rec.Body.String()) != tt.wantBody {
			t.Fatalf("%s body=%q want %q", tt.path, strings.TrimSpace(rec.Body.String()), tt.wantBody)
		}
	}
}

func TestStaticFilesDoesNotOpenDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	fileSystem := StaticFiles(root)
	if file, err := fileSystem.Open("/file.txt"); err != nil {
		t.Fatalf("Open(file.txt) err=%v", err)
	} else {
		_ = file.Close()
	}

	if file, err := fileSystem.Open("/"); err == nil {
		_ = file.Close()
		t.Fatal("Open(/) succeeded, want error")
	} else if !os.IsNotExist(err) {
		t.Fatalf("Open(/) err=%v, want not exist", err)
	}
}

func TestHandleSPAKeepsNotFoundPrefixes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := gin.New()
	HandleSPA(engine, SPAOptions{
		Root: root,
		DirOptions: DirOptions{
			ShowList:  false,
			SPA:       true,
			IndexName: "index.html",
		},
		NotFoundPrefixes: []string{"/api/"},
	})

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/", wantStatus: http.StatusOK, wantBody: "<html>spa</html>"},
		{path: "/support", wantStatus: http.StatusOK, wantBody: "<html>spa</html>"},
		{path: "/api/not-exists", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

		if rec.Code != tt.wantStatus {
			t.Fatalf("%s status=%d want %d", tt.path, rec.Code, tt.wantStatus)
		}
		if tt.wantBody != "" && strings.TrimSpace(rec.Body.String()) != tt.wantBody {
			t.Fatalf("%s body=%q want %q", tt.path, strings.TrimSpace(rec.Body.String()), tt.wantBody)
		}
	}
}

func TestHandleSPARootRedirect(t *testing.T) {
	// Mirrors the real export: no index.html at the root, only the exported
	// route shells.
	engine := gin.New()
	HandleSPA(engine, SPAOptions{
		EmbeddedFS: fstest.MapFS{
			"dashboard.html": {Data: []byte("<html>dashboard</html>")},
			"support.html":   {Data: []byte("<html>support</html>")},
		},
		EmbeddedRoot:     ".",
		RootRedirect:     "/support",
		DirOptions:       DirOptions{ShowList: false, SPA: true, IndexName: "index.html"},
		NotFoundPrefixes: []string{"/api/"},
	})

	t.Run("root redirects", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(method, "/", nil))

			if rec.Code != http.StatusFound {
				t.Fatalf("%s / status=%d want %d", method, rec.Code, http.StatusFound)
			}
			if location := rec.Header().Get("Location"); location != "/support" {
				t.Fatalf("%s / Location=%q want %q", method, location, "/support")
			}
		}
	})

	t.Run("other routes still serve the SPA", func(t *testing.T) {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/support", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("/support status=%d want %d", rec.Code, http.StatusOK)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != "<html>support</html>" {
			t.Fatalf("/support body=%q", body)
		}
	})

	t.Run("api 404 is unaffected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/not-exists", nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("/api/not-exists status=%d want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestDirHandlerWithNextExportPages(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>home</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dashboard.html"), []byte("<html>dashboard</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	dashboardDir := filepath.Join(root, "dashboard")
	if err := os.MkdirAll(dashboardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dashboardDir, "login.html"), []byte("<html>login</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := DirHandler(http.Dir(root), DirOptions{
		ShowList:  false,
		SPA:       true,
		IndexName: "index.html",
	})

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/", wantStatus: http.StatusOK, wantBody: "<html>home</html>"},
		{path: "/dashboard", wantStatus: http.StatusOK, wantBody: "<html>dashboard</html>"},
		{path: "/dashboard/", wantStatus: http.StatusOK, wantBody: "<html>dashboard</html>"},
		{path: "/dashboard/login", wantStatus: http.StatusOK, wantBody: "<html>login</html>"},
		{path: "/dashboard/login/", wantStatus: http.StatusOK, wantBody: "<html>login</html>"},
		{path: "/other-spa-route", wantStatus: http.StatusOK, wantBody: "<html>home</html>"},
	}

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)

		handler(ctx)

		if rec.Code != tt.wantStatus {
			t.Fatalf("%s status=%d want %d", tt.path, rec.Code, tt.wantStatus)
		}
		if tt.wantBody != "" && strings.TrimSpace(rec.Body.String()) != tt.wantBody {
			t.Fatalf("%s body=%q want %q", tt.path, strings.TrimSpace(rec.Body.String()), tt.wantBody)
		}
	}
}
