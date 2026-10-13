package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/app"
	"github.com/zalando/go-keyring"
)

func cmdLogout(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	profile := fs.String("profile", "", "profile to delete")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	if *profile != "" {
		cfg.Profile = *profile
	}
	if err := cfg.Store().Delete(cfg.Profile); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "Deleted profile %q.\n", cfg.Profile)
	return 0
}

func cmdStatus(stdout, stderr io.Writer) int {
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	store := cfg.Store()
	names, err := store.List()
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "config dir: %s\nactive profile: %s\n", cfg.ConfigDir, cfg.Profile)
	if len(names) == 0 {
		fmt.Fprintln(stdout, "no profiles; run: sourcebolt login --import <cookies file>")
		return 0
	}
	for _, n := range names {
		p, err := store.Load(n)
		if err != nil {
			fmt.Fprintf(stdout, "  %-12s unreadable: %v\n", n, err)
			continue
		}
		key := "os keyring"
		if p.KeyInFile {
			key = "key file"
		}
		fmt.Fprintf(stdout, "  %-12s host=%s updated=%s key=%s valid_cookies=%v\n",
			n, p.Host, p.Updated.Local().Format(time.RFC3339), key, p.Jar.Valid())
	}
	return 0
}

func cmdDoctor(ctx context.Context, stdout, stderr io.Writer) int {
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	failed := false
	check := func(name string, err error) {
		if err != nil {
			failed = true
			fmt.Fprintf(stdout, "FAIL  %-22s %v\n", name, err)
			return
		}
		fmt.Fprintf(stdout, "ok    %s\n", name)
	}

	check("config dir writable", func() error {
		if err := os.MkdirAll(cfg.ConfigDir, 0o700); err != nil {
			return err
		}
		f, err := os.CreateTemp(cfg.ConfigDir, "doctor-*")
		if err != nil {
			return err
		}
		f.Close()
		return os.Remove(f.Name())
	}())
	check("os keyring", func() error {
		const svc, user = "sourcebolt-doctor", "probe"
		if err := keyring.Set(svc, user, "x"); err != nil {
			return fmt.Errorf("%w (profiles fall back to a 0600 key file)", err)
		}
		return keyring.Delete(svc, user)
	}())

	rt, err := app.Build(cfg)
	check("profile "+cfg.Profile, err)
	if err == nil {
		if !rt.Profile.Jar.Valid() {
			check("cookies", errors.New("no valid session; run: sourcebolt login --import <cookies file>"))
		} else {
			wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			start := time.Now()
			err := rt.Transport.Warm(wctx)
			cancel()
			check(fmt.Sprintf("token bootstrap %s (%v)", rt.Profile.Host, time.Since(start).Round(time.Millisecond)), err)
		}
	}
	fmt.Fprintf(stdout, "profiles dir: %s\n", filepath.Join(cfg.ConfigDir, "profiles"))
	if failed {
		return 1
	}
	return 0
}
