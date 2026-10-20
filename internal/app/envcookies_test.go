package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/zalando/go-keyring"
)

// Obvious fakes; none of these is a real Google cookie.
const (
	fakeHeader = "Cookie: SID=fake-env-sid-111; __Secure-1PSIDTS=fake-env-ts-222; OSID=fake-env-osid-333"

	fakeNetscape = "# Netscape HTTP Cookie File\n" +
		".google.com\tTRUE\t/\tTRUE\t0\tSID\tfake-env-sid-111\n" +
		".google.com\tTRUE\t/\tTRUE\t0\t__Secure-1PSIDTS\tfake-env-ts-222\n" +
		".google.com\tTRUE\t/\tTRUE\t0\tAPISID\tfake-env-apisid-444\n" +
		".google.com\tTRUE\t/\tTRUE\t0\tSAPISID\tfake-env-sapisid-555\n"

	fakeJSON = `[{"name":"SID","value":"fake-env-sid-111","domain":".google.com","path":"/"},` +
		`{"name":"__Secure-1PSIDTS","value":"fake-env-ts-222","domain":".google.com","path":"/"},` +
		`{"name":"OSID","value":"fake-env-osid-333","domain":"notebook.google.com","path":"/"}]`
)

// envTest isolates a test: temp config dir, no browser, no cookie env
// vars left over from the developer's shell, and the in-memory keyring.
func envTest(t *testing.T) Config {
	t.Helper()
	keyring.MockInit()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	t.Setenv("NOTEBOOKLM_PROFILE", "")
	t.Setenv("NOTEBOOKLM_HOST", "")
	t.Setenv(EnvCookies, "")
	t.Setenv(EnvCookiesFile, "")
	os.Unsetenv(EnvCookies)
	os.Unsetenv(EnvCookiesFile)
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func cookieValue(j *session.Jar, name string) string {
	for _, c := range j.All() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func assertEnvRuntime(t *testing.T, cfg Config, rt *Runtime, source string) {
	t.Helper()
	if !rt.Profile.Jar.Valid() {
		t.Fatalf("env jar not valid, missing %v", rt.Profile.Jar.Missing())
	}
	if got := cookieValue(rt.Profile.Jar, "SID"); got != "fake-env-sid-111" {
		t.Fatalf("SID = %q, want the env value", got)
	}
	if rt.CookieSource != source {
		t.Fatalf("CookieSource = %q, want %q", rt.CookieSource, source)
	}
	if rt.Store != nil {
		t.Fatal("env profile has a store: rotated cookies would be written to disk")
	}
	if rt.Profile.Name != cfg.Profile || rt.Profile.Host != session.HostDefault {
		t.Fatalf("profile = %q on %q", rt.Profile.Name, rt.Profile.Host)
	}
	if files, _ := filepath.Glob(filepath.Join(cfg.ConfigDir, "profiles", "*")); len(files) != 0 {
		t.Fatalf("env cookies were written to disk: %v", files)
	}
}

func TestEnvCookiesFormats(t *testing.T) {
	for name, raw := range map[string]string{"header": fakeHeader, "netscape": fakeNetscape, "json": fakeJSON} {
		t.Run(name, func(t *testing.T) {
			cfg := envTest(t)
			t.Setenv(EnvCookies, raw)
			rt, err := Build(cfg)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			defer rt.Close()
			assertEnvRuntime(t, cfg, rt, EnvCookies)
		})
	}
}

func TestEnvCookiesFile(t *testing.T) {
	cfg := envTest(t)
	path := filepath.Join(t.TempDir(), "notebooklm_cookies")
	if err := os.WriteFile(path, []byte(fakeNetscape), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvCookiesFile, path)
	rt, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer rt.Close()
	assertEnvRuntime(t, cfg, rt, EnvCookiesFile)
}

func TestEnvCookiesFileUnreadable(t *testing.T) {
	cfg := envTest(t)
	t.Setenv(EnvCookiesFile, filepath.Join(t.TempDir(), "absent"))
	if _, err := Build(cfg); err == nil || !strings.Contains(err.Error(), EnvCookiesFile) {
		t.Fatalf("err = %v, want one naming %s", err, EnvCookiesFile)
	}
}

func TestEnvCookiesMissingRequired(t *testing.T) {
	cfg := envTest(t)
	t.Setenv(EnvCookies, "SID=fake-lonely-sid-999")
	_, err := Build(cfg)
	if err == nil {
		t.Fatal("Build accepted a jar without the required cookies")
	}
	msg := err.Error()
	for _, want := range []string{EnvCookies, "missing required cookies", "__Secure-1PSIDTS", "OSID or APISID+SAPISID"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error lacks %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "fake-lonely-sid-999") {
		t.Errorf("error leaks a cookie value: %s", msg)
	}
}

// No error may carry a cookie value, whatever went wrong.
func TestEnvCookiesErrorsNeverHoldValues(t *testing.T) {
	const secret = "fake-secret-zzz-777"
	cases := map[string]string{
		"broken json":  `[{"name":"SID","value":"` + secret + `"`,
		"missing":      "Cookie: SID=" + secret + "; HSID=" + secret,
		"no cookies":   secret,
		"netscape bad": ".google.com\tTRUE\t/\tTRUE\t0\tHSID\t" + secret + "\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := envTest(t)
			t.Setenv(EnvCookies, raw)
			_, err := Build(cfg)
			if err == nil {
				t.Fatal("Build accepted bad cookies")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error leaks a cookie value: %s", err)
			}
		})
	}
}

func TestEnvCookiesUnsetAfterRead(t *testing.T) {
	for name, raw := range map[string]string{"valid": fakeHeader, "invalid": "SID=fake-x"} {
		t.Run(name, func(t *testing.T) {
			cfg := envTest(t)
			path := filepath.Join(t.TempDir(), "c")
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(EnvCookies, raw)
			rt, err := Build(cfg)
			if rt != nil {
				rt.Close()
			}
			_ = err
			if _, ok := os.LookupEnv(EnvCookies); ok {
				t.Fatalf("%s still set after Build (child processes would inherit it)", EnvCookies)
			}

			t.Setenv(EnvCookiesFile, path)
			rt, _ = Build(cfg)
			if rt != nil {
				rt.Close()
			}
			if _, ok := os.LookupEnv(EnvCookiesFile); ok {
				t.Fatalf("%s still set after Build", EnvCookiesFile)
			}
		})
	}
}

func TestEnvCookiesBothSetIsAnError(t *testing.T) {
	cfg := envTest(t)
	path := filepath.Join(t.TempDir(), "c")
	if err := os.WriteFile(path, []byte(fakeHeader), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvCookies, fakeHeader)
	t.Setenv(EnvCookiesFile, path)
	_, err := Build(cfg)
	if err == nil || !strings.Contains(err.Error(), EnvCookies) || !strings.Contains(err.Error(), EnvCookiesFile) {
		t.Fatalf("err = %v, want one naming both variables", err)
	}
}

// Env cookies, when set, win over a saved profile; the saved profile is
// never touched, so unsetting the variable brings it back.
func TestEnvCookiesWinOverSavedProfile(t *testing.T) {
	cfg := envTest(t)
	saved := session.NewJar()
	for _, c := range []session.Cookie{
		{Name: "SID", Value: "fake-saved-sid", Domain: ".google.com"},
		{Name: "__Secure-1PSIDTS", Value: "fake-saved-ts", Domain: ".google.com"},
		{Name: "OSID", Value: "fake-saved-osid", Domain: "notebook.google.com"},
	} {
		saved.Add(c)
	}
	if err := cfg.Store().Save(&session.Profile{Name: cfg.Profile, Host: session.HostDefault, Jar: saved}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(cfg.ConfigDir, "profiles", cfg.Profile+".profile"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv(EnvCookies, fakeHeader)
	rt, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := cookieValue(rt.Profile.Jar, "SID"); got != "fake-env-sid-111" {
		t.Fatalf("SID = %q, want the env value over the saved one", got)
	}
	if rt.Store != nil || rt.CookieSource != EnvCookies {
		t.Fatalf("Store = %v, CookieSource = %q", rt.Store, rt.CookieSource)
	}
	rt.Close()
	after, _ := os.ReadFile(filepath.Join(cfg.ConfigDir, "profiles", cfg.Profile+".profile"))
	if string(before) != string(after) {
		t.Fatal("the saved profile changed while env cookies were active")
	}

	rt, err = Build(cfg) // the variable was unset by the first Build
	if err != nil {
		t.Fatalf("Build without env: %v", err)
	}
	defer rt.Close()
	if got := cookieValue(rt.Profile.Jar, "SID"); got != "fake-saved-sid" || rt.CookieSource != "" || rt.Store == nil {
		t.Fatalf("without env: SID = %q, CookieSource = %q, Store = %v", got, rt.CookieSource, rt.Store)
	}
}

func TestEnvCookiesHonourHostOverride(t *testing.T) {
	cfg := envTest(t)
	cfg.Host = "notebooklm.google.com"
	t.Setenv(EnvCookies, fakeHeader)
	rt, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer rt.Close()
	if rt.Profile.Host != "notebooklm.google.com" {
		t.Fatalf("Host = %q", rt.Profile.Host)
	}
}
