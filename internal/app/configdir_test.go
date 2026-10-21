package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/zalando/go-keyring"
)

// fakeDirs points the default folders at temp dirs, so these tests never
// read the real home or %APPDATA%.
func fakeDirs(t *testing.T, goos string) (home, appdata string) {
	t.Helper()
	root := t.TempDir()
	home, appdata = filepath.Join(root, "home"), filepath.Join(root, "home", "AppData", "Roaming")
	old := dirEnv
	dirEnv = func() session.DirEnv {
		return session.DirEnv{GOOS: goos,
			Home:   func() (string, error) { return home, nil },
			Config: func() (string, error) { return appdata, nil }}
	}
	t.Cleanup(func() { dirEnv = old })
	keyring.MockInit()
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", "")
	return home, appdata
}

func oldProfile(t *testing.T, appdata string) {
	t.Helper()
	p := &session.Profile{Name: "default", Host: session.HostDefault, Jar: session.NewJar()}
	p.Jar.Add(session.Cookie{Name: "SID", Value: "fake-sid", Domain: ".google.com", Path: "/"})
	if err := session.NewFileStore(filepath.Join(appdata, "sourcebolt", "profiles"), session.OSKeyring{}).Save(p); err != nil {
		t.Fatal(err)
	}
}

func TestConfigDirWindowsDefault(t *testing.T) {
	home, _ := fakeDirs(t, "windows")
	c, err := ConfigFromEnv()
	if err != nil || c.ConfigDir != filepath.Join(home, ".sourcebolt") {
		t.Fatalf("ConfigDir = %q, %v; want %q", c.ConfigDir, err, filepath.Join(home, ".sourcebolt"))
	}
}

func TestConfigDirOtherSystemsUnchanged(t *testing.T) {
	_, appdata := fakeDirs(t, "linux")
	c, err := ConfigFromEnv()
	if err != nil || c.ConfigDir != filepath.Join(appdata, "sourcebolt") {
		t.Fatalf("ConfigDir = %q, %v", c.ConfigDir, err)
	}
}

func TestStoreMigratesFromTheOldWindowsFolder(t *testing.T) {
	home, appdata := fakeDirs(t, "windows")
	oldProfile(t, appdata)
	c, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Store().Load("default"); err != nil {
		t.Fatalf("profile not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".sourcebolt", "profiles", "default.profile")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(appdata, "sourcebolt", "profiles", "default.profile")); err != nil {
		t.Fatalf("old profile must stay: %v", err)
	}
}

func TestConfigDirEnvDisablesMigration(t *testing.T) {
	home, appdata := fakeDirs(t, "windows")
	oldProfile(t, appdata)
	custom := t.TempDir()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", custom)
	c, err := ConfigFromEnv()
	if err != nil || c.ConfigDir != custom {
		t.Fatalf("ConfigDir = %q, %v", c.ConfigDir, err)
	}
	if _, err := c.Store().Load("default"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("NOTEBOOKLM_CONFIG_DIR must disable migration; Load = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".sourcebolt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the default folder was created although NOTEBOOKLM_CONFIG_DIR is set")
	}
}
