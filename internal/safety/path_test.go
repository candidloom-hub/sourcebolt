package safety

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func mkRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCheckPathAllowed(t *testing.T) {
	root := mkRoot(t)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, in string }{
		{"existing file", filepath.Join(root, "sub", "a.txt")},
		{"not yet existing file", filepath.Join(root, "sub", "new.txt")},
		{"deep nonexistent", filepath.Join(root, "x", "y", "z.txt")},
		{"root itself", root},
		{"redundant separators", root + string(filepath.Separator) + string(filepath.Separator) + "sub" + string(filepath.Separator) + "a.txt"},
		{"dot segment", filepath.Join(root, ".", "sub", "a.txt")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CheckPath(c.in, []string{root})
			if err != nil {
				t.Fatal(err)
			}
			if !filepath.IsAbs(got) || !strings.HasPrefix(strings.ToLower(got), strings.ToLower(realRoot)) {
				t.Errorf("got %q, want absolute path under %q", got, realRoot)
			}
		})
	}
}

func TestCheckPathSecondRoot(t *testing.T) {
	a, b := mkRoot(t), mkRoot(t)
	if _, err := CheckPath(filepath.Join(b, "sub", "a.txt"), []string{a, b}); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPathRejected(t *testing.T) {
	root := mkRoot(t)
	other := mkRoot(t)
	sep := string(filepath.Separator)
	cases := []struct{ name, in string }{
		{"empty", ""},
		{"relative", filepath.Join("sub", "a.txt")},
		{"dotdot escape", root + sep + ".." + sep + filepath.Base(other) + sep + "sub"},
		{"dotdot to parent", root + sep + ".."},
		{"dotdot inside then out", filepath.Join(root, "sub") + sep + ".." + sep + ".." + sep + "etc"},
		{"outside root", filepath.Join(other, "sub", "a.txt")},
		{"sibling with shared prefix", root + "-evil" + sep + "a.txt"},
		{"nul byte", filepath.Join(root, "a\x00.txt")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := CheckPath(c.in, []string{root}); err == nil {
				t.Fatalf("want rejection, got %q", got)
			}
		})
	}
}

func TestCheckPathNoRoots(t *testing.T) {
	root := mkRoot(t)
	if _, err := CheckPath(root, nil); err == nil {
		t.Fatal("want rejection with no roots")
	}
}

func TestCheckPathSymlinkEscape(t *testing.T) {
	root, outside := mkRoot(t), mkRoot(t)
	link := filepath.Join(root, "link")
	if err := mkLink(outside, link); err != nil {
		t.Skipf("cannot create symlink here: %v", err)
	}
	for _, in := range []string{
		link,
		filepath.Join(link, "sub", "a.txt"),
		filepath.Join(link, "not-there.txt"),
	} {
		if got, err := CheckPath(in, []string{root}); err == nil {
			t.Errorf("%q: want rejection, got %q", in, got)
		}
	}
}

func TestCheckPathSymlinkInside(t *testing.T) {
	root := mkRoot(t)
	link := filepath.Join(root, "inlink")
	if err := mkLink(filepath.Join(root, "sub"), link); err != nil {
		t.Skipf("cannot create symlink here: %v", err)
	}
	skipIfJunction(t, link)
	got, err := CheckPath(filepath.Join(link, "a.txt"), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, filepath.Join("sub", "a.txt")) {
		t.Errorf("symlink not resolved: %q", got)
	}
}

func TestCheckPathRootIsSymlink(t *testing.T) {
	real := mkRoot(t)
	linkRoot := filepath.Join(t.TempDir(), "rootlink")
	if err := mkLink(real, linkRoot); err != nil {
		t.Skipf("cannot create symlink here: %v", err)
	}
	skipIfJunction(t, linkRoot)
	if _, err := CheckPath(filepath.Join(linkRoot, "sub", "a.txt"), []string{linkRoot}); err != nil {
		t.Fatal(err)
	}
}

// mkLink creates a symlink, falling back to a directory junction on Windows
// where symlink creation needs a privilege. Junctions are resolved by
// EvalSymlinks too, so they exercise the same escape logic.
func mkLink(target, link string) error {
	err := os.Symlink(target, link)
	if err == nil || runtime.GOOS != "windows" {
		return err
	}
	if out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr != nil {
		return fmt.Errorf("symlink: %v; junction: %v: %s", err, jerr, out)
	}
	return nil
}

// skipIfJunction skips tests that expect a link to be followed: CheckPath
// deliberately rejects Windows junctions, so a junction fallback cannot
// stand in for a real symlink there.
func skipIfJunction(t *testing.T, link string) {
	t.Helper()
	if fi, err := os.Lstat(link); err == nil && fi.Mode()&os.ModeSymlink == 0 {
		t.Skip("only a junction is available; CheckPath rejects junctions by design")
	}
}

func TestCheckPathWindowsOnly(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows path rules")
	}
	root := mkRoot(t)
	cases := []struct{ name, in string }{
		{"device namespace", `\\.\C:\Windows\x`},
		{"device pipe", `\\.\pipe\foo`},
		{"extended prefix", `\\?\C:\Windows\x`},
		{"extended prefix in root", `\\?\` + root + `\sub\a.txt`},
		{"UNC", `\\server\share\x`},
		{"UNC forward slashes", `//server/share/x`},
		{"CON", filepath.Join(root, "CON")},
		{"nul with ext", filepath.Join(root, "nul.txt")},
		{"com1 lower", filepath.Join(root, "sub", "com1")},
		{"LPT9", filepath.Join(root, "LPT9.log")},
		{"CONIN$", filepath.Join(root, "CONIN$")},
		{"ADS", filepath.Join(root, "sub", "a.txt:stream")},
		{"ADS with type", filepath.Join(root, "sub", "a.txt:stream:$DATA")},
		{"trailing dot", filepath.Join(root, "sub", "a.txt.")},
		{"trailing space", filepath.Join(root, "sub", "a.txt ")},
		{"drive relative", `C:foo\bar`},
		{"rooted no drive", `\Windows\x`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := CheckPath(c.in, []string{root}); err == nil {
				t.Fatalf("want rejection, got %q", got)
			}
		})
	}
	t.Run("lookalike names allowed", func(t *testing.T) {
		for _, n := range []string{"console.txt", "com10", "nullable", "aux2"} {
			if _, err := CheckPath(filepath.Join(root, n), []string{root}); err != nil {
				t.Errorf("%s: %v", n, err)
			}
		}
	})
	t.Run("case-insensitive root", func(t *testing.T) {
		in := strings.ToUpper(filepath.Join(root, "sub", "a.txt"))
		if _, err := CheckPath(in, []string{strings.ToLower(root)}); err != nil {
			t.Fatal(err)
		}
	})
}
