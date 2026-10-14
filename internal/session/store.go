package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// Profile is one signed-in Google account.
type Profile struct {
	Name      string
	Host      Host
	Jar       *Jar
	Rev       int64 // revision on disk; Save fails if someone else saved since Load
	Updated   time.Time
	KeyInFile bool // the encryption key lives in a 0600 file (no OS keyring)
}

// Store persists profiles.
type Store interface {
	Load(name string) (*Profile, error)
	Save(p *Profile) error
	Delete(name string) error
	List() ([]string, error)
}

// Keyring is the subset of an OS keyring the store needs.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}

// OSKeyring uses the platform keyring (Windows Credential Manager, macOS
// Keychain, Linux Secret Service). Only a 32-byte key goes there: Windows
// limits entries to 2,560 bytes, too small for a cookie jar.
type OSKeyring struct{}

func (OSKeyring) Get(s, u string) (string, error) { return keyring.Get(s, u) }
func (OSKeyring) Set(s, u, v string) error        { return keyring.Set(s, u, v) }
func (OSKeyring) Delete(s, u string) error        { return keyring.Delete(s, u) }

var ErrStaleRevision = errors.New("session: profile changed on disk since it was loaded")

const (
	keyringService = "sourcebolt"
	fileVersion    = 1
	profileExt     = ".profile"
	keyExt         = ".key"
)

// beforeRename is a test hook that runs between writing the temp file and
// renaming it into place.
var beforeRename func() error

type fileStore struct {
	dir string
	kr  Keyring
}

// NewFileStore stores each profile as an AES-256-GCM encrypted file in dir.
func NewFileStore(dir string, kr Keyring) Store { return &fileStore{dir: dir, kr: kr} }

// DefaultDir is <user config dir>/sourcebolt/profiles.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sourcebolt", "profiles"), nil
}

type diskProfile struct {
	Host    Host      `json:"host"`
	Cookies []Cookie  `json:"cookies"`
	Rev     int64     `json:"rev"`
	Updated time.Time `json:"updated"`
}

func (s *fileStore) path(name, ext string) string { return filepath.Join(s.dir, name+ext) }

// pathName is the rule every store operation enforces: the name is one
// path element. Load and Delete use only this, so profiles saved before
// validName got stricter can still be read and removed.
func pathName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\:*?"<>|`) || strings.HasPrefix(name, ".") ||
		strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return fmt.Errorf("session: invalid profile name %q", name)
	}
	return nil
}

// validName rejects profile names that are unsafe as file names on any
// OS: path and wildcard characters, control characters, a leading dot or
// space, a trailing dot or space (Windows path APIs strip them), and Windows device
// names (CON, NUL, COM1 ...), which stay reserved with an extension.
func validName(name string) error {
	bad := name == "" || strings.ContainsAny(name, `/\:*?"<>|`) ||
		strings.HasPrefix(name, ".") || strings.HasPrefix(name, " ") ||
		strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") ||
		strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) ||
		reservedDevice(name)
	if bad {
		return fmt.Errorf("session: invalid profile name %q", name)
	}
	return nil
}

func reservedDevice(name string) bool {
	base, _, _ := strings.Cut(name, ".")
	switch strings.ToUpper(base) {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) < 4 {
		return false
	}
	switch strings.ToUpper(base[:3]) {
	case "COM", "LPT":
		switch base[3:] {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
			return true
		}
	}
	return false
}

func (s *fileStore) Load(name string) (*Profile, error) {
	if err := pathName(name); err != nil {
		return nil, err
	}
	blob, err := os.ReadFile(s.path(name, profileExt))
	if err != nil {
		return nil, err
	}
	key, inFile, err := s.getKey(name)
	if err != nil {
		return nil, err
	}
	dp, err := decrypt(key, blob)
	if err != nil {
		return nil, err
	}
	j := NewJar()
	for _, c := range dp.Cookies {
		j.Add(c)
	}
	return &Profile{Name: name, Host: dp.Host, Jar: j, Rev: dp.Rev, Updated: dp.Updated, KeyInFile: inFile}, nil
}

// Save refuses a new profile whose name fails validName. A profile already
// on disk under a name the older, looser rule allowed (e.g. "work ") is
// still updated, so cookie rotation keeps working for it.
func (s *fileStore) Save(p *Profile) error {
	if err := validName(p.Name); err != nil {
		if pathName(p.Name) != nil {
			return err
		}
		if _, serr := os.Stat(s.path(p.Name, profileExt)); serr != nil {
			return err
		}
	}
	return s.save(p)
}

func (s *fileStore) save(p *Profile) error {
	if err := pathName(p.Name); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	unlock, err := s.lock(p.Name)
	if err != nil {
		return err
	}
	defer unlock()
	_, statErr := os.Stat(s.path(p.Name, profileExt))
	exists := statErr == nil
	key, inFile, err := s.getKey(p.Name)
	if err != nil && !exists {
		// Mint a key only for a brand-new profile. For an existing profile a
		// missing or unreadable key must never be replaced: that would make
		// the saved cookies undecryptable forever.
		key, inFile, err = s.newKey(p.Name)
	}
	if err != nil {
		return err
	}
	if blob, err := os.ReadFile(s.path(p.Name, profileExt)); err == nil {
		cur, err := decrypt(key, blob)
		if err != nil {
			return err
		}
		if cur.Rev != p.Rev {
			return ErrStaleRevision
		}
	}
	dp := diskProfile{Host: p.Host, Cookies: p.Jar.All(), Rev: p.Rev + 1, Updated: time.Now().UTC()}
	blob, err := encrypt(key, dp)
	if err != nil {
		return err
	}
	if err := writeAtomic(s.path(p.Name, profileExt), blob); err != nil {
		return err
	}
	p.Rev, p.Updated, p.KeyInFile = dp.Rev, dp.Updated, inFile
	return nil
}

func (s *fileStore) Delete(name string) error {
	if err := pathName(name); err != nil {
		return err
	}
	_ = s.kr.Delete(keyringService, name)
	for _, ext := range []string{profileExt, keyExt} {
		if err := os.Remove(s.path(name, ext)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *fileStore) List() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(s.dir, "*"+profileExt))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, strings.TrimSuffix(filepath.Base(f), profileExt))
	}
	return names, nil
}

// getKey returns the profile key from the keyring, else from the key file.
// A keyring failure other than "not found" is returned as is, so callers
// never mistake a locked keyring for a missing key.
func (s *fileStore) getKey(name string) ([]byte, bool, error) {
	v, kerr := s.kr.Get(keyringService, name)
	if kerr == nil {
		k, err := base64.StdEncoding.DecodeString(v)
		return k, false, err
	}
	b, err := os.ReadFile(s.path(name, keyExt))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !errors.Is(kerr, keyring.ErrNotFound) {
			return nil, false, fmt.Errorf("session: keyring: %w", kerr)
		}
		return nil, true, err
	}
	k, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	return k, true, err
}

func (s *fileStore) newKey(name string) ([]byte, bool, error) {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, false, err
	}
	enc := base64.StdEncoding.EncodeToString(k)
	if err := s.kr.Set(keyringService, name, enc); err == nil {
		return k, false, nil
	}
	if err := writeAtomic(s.path(name, keyExt), []byte(enc)); err != nil {
		return nil, true, err
	}
	return k, true, nil
}

func encrypt(key []byte, dp diskProfile) ([]byte, error) {
	plain, err := json.Marshal(dp)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{fileVersion}, nonce...)
	return gcm.Seal(out, nonce, plain, []byte{fileVersion}), nil
}

func decrypt(key, blob []byte) (diskProfile, error) {
	var dp diskProfile
	gcm, err := newGCM(key)
	if err != nil {
		return dp, err
	}
	if len(blob) < 1+gcm.NonceSize() || blob[0] != fileVersion {
		return dp, errors.New("session: unsupported profile file")
	}
	nonce, ct := blob[1:1+gcm.NonceSize()], blob[1+gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, []byte{fileVersion})
	if err != nil {
		return dp, fmt.Errorf("session: decrypt profile: %w", err)
	}
	return dp, json.Unmarshal(plain, &dp)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// writeAtomic writes via a temp file in the same directory, then renames.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	if beforeRename != nil {
		if err := beforeRename(); err != nil {
			return err
		}
	}
	return renameRetry(tmpName, path)
}

var (
	osRename = os.Rename
	renameFn = os.Rename // test hook
)

// renameRetry retries briefly when Windows reports the target as in use
// (another process has it open), instead of silently losing the save.
func renameRetry(from, to string) error {
	var err error
	for range 20 {
		if err = renameFn(from, to); err == nil || !errors.Is(err, fs.ErrPermission) {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
	return err
}

const (
	lockWait  = 3 * time.Second
	lockStale = 15 * time.Second
)

// lock serializes check-and-write across processes with an O_EXCL lock file.
func (s *fileStore) lock(name string) (func(), error) {
	path := s.path(name, ".lock")
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if st, serr := os.Stat(path); serr == nil && time.Since(st.ModTime()) > lockStale {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("session: profile %q is locked by another process", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
