package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kido5217/yolo/internal/protocol"
)

// Profile selection and lifecycle (a yolo extension, no upstream opencode
// counterpart): a profile is a directory <root>/<id>/ holding the usual
// global config files; the active profile id is recorded in the one-line
// <root>/active marker.

const (
	activeFileName   = "active"
	defaultProfileID = "default"
)

var (
	// ErrNotFound: no profile matches the reference.
	ErrNotFound = errors.New("config: profile not found")
	// ErrAmbiguous: the reference matches the name of several profiles.
	ErrAmbiguous = errors.New("config: profile name is ambiguous")
	// ErrNameTaken: the profile name is already used by another profile.
	ErrNameTaken = errors.New("config: profile name already in use")
)

// Profile is one profile on disk: ID is the directory name, Name is the
// effective display name (the "profile" element name, falling back to ID),
// Description the element's optional description.
type Profile struct {
	ID          string
	Name        string
	Description string
}

// ProcessProfile resolves the profile id for a process against root: the
// flag reference (id or name) beats the YOLO_PROFILE env value (env nil =
// os environment), which beats the active marker (EnsureActive creates the
// default profile on a first run).
func ProcessProfile(root, flag string, env map[string]string) (string, error) {
	if flag != "" {
		return Resolve(root, flag)
	}
	envVal := func(name string) (string, bool) {
		if env != nil {
			v, ok := env[name]
			return v, ok
		}
		return os.LookupEnv(name)
	}
	if v, ok := envVal(ProfileEnvVar); ok && v != "" {
		return Resolve(root, v)
	}
	return EnsureActive(root)
}

// GenerateID returns a fresh profile id: 8 lowercase hex chars from
// crypto/rand (Kubernetes-style short random suffix).
func GenerateID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating profile id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func activePath(root string) string {
	return filepath.Join(root, activeFileName)
}

// ActiveID reads the active marker; a missing marker (or missing root) is
// the empty string, not an error.
func ActiveID(root string) (string, error) {
	b, err := os.ReadFile(activePath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading active profile: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// SetActive records id in the active marker after verifying the profile
// directory exists.
func SetActive(root, id string) error {
	if err := checkProfileDir(root, id); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	return os.WriteFile(activePath(root), []byte(id+"\n"), 0o644)
}

// EnsureActive guarantees root holds a usable active profile: a fresh
// root gets a "default" profile, and a missing or stale marker recovers
// to "default" (created if absent, never clobbered). Returns the active id.
func EnsureActive(root string) (string, error) {
	id, err := ActiveID(root)
	if err != nil {
		return "", err
	}
	if id != "" {
		if st, err := os.Stat(filepath.Join(root, id)); err == nil && st.IsDir() {
			return id, nil
		}
	}
	if err := os.MkdirAll(filepath.Join(root, defaultProfileID), 0o755); err != nil {
		return "", err
	}
	if err := SetActive(root, defaultProfileID); err != nil {
		return "", err
	}
	return defaultProfileID, nil
}

// List returns every profile under root sorted by effective name (then id),
// with metadata from each profile's merged global config. A missing root is
// an empty list; the active marker is not a profile. A profile whose config
// fails to load still lists — with its id as the name and a blank
// description — so one corrupt profile can never break the listing (or the
// name checks built on it).
func List(root string) ([]Profile, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []Profile{}, nil
		}
		return nil, err
	}
	out := make([]Profile, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfg, err := LoadGlobal(filepath.Join(root, e.Name()))
		if err != nil {
			out = append(out, Profile{ID: e.Name(), Name: e.Name()})
			continue
		}
		name, desc := e.Name(), ""
		if cfg.Profile != nil {
			if cfg.Profile.Name != "" {
				name = cfg.Profile.Name
			}
			desc = cfg.Profile.Description
		}
		out = append(out, Profile{ID: e.Name(), Name: name, Description: desc})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Resolve maps a profile reference to an id: an exact id (directory name)
// match wins; otherwise an exact effective-name match (a name shared by
// several profiles is ErrAmbiguous, none is ErrNotFound). A profile whose
// config fails to load is skipped in name matching — its id still matches
// exactly, and a corrupt sibling can never block a healthy one.
func Resolve(root, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("%w: empty reference", ErrNotFound)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
		}
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && e.Name() == ref {
			return e.Name(), nil
		}
	}
	matches := make([]string, 0, 1)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfg, err := LoadGlobal(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		if cfg.Profile != nil && cfg.Profile.Name == ref {
			matches = append(matches, e.Name())
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%w: %q matches %d profiles", ErrAmbiguous, ref, len(matches))
	}
}

// Add creates a new profile with an auto-generated id. name and description
// (both optional) are stored in the new profile's yolo.jsonc "profile"
// element (mode 0600 — the config may carry provider API keys); a
// duplicate effective name is ErrNameTaken.
func Add(root, name, description string) (Profile, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if err := checkNameFree(root, name); err != nil {
		return Profile{}, err
	}
	id, err := uniqueID(root)
	if err != nil {
		return Profile{}, err
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Profile{}, err
	}
	if name != "" || description != "" {
		prof := map[string]any{}
		if name != "" {
			prof["name"] = name
		}
		if description != "" {
			prof["description"] = description
		}
		b, err := json.MarshalIndent(map[string]any{"profile": prof}, "", "  ")
		if err != nil {
			return Profile{}, err
		}
		p := filepath.Join(dir, yoloFileJSONC)
		if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
			return Profile{}, err
		}
		// 0600: the config may carry provider API keys; the explicit
		// Chmod also upgrades a pre-existing 0644 file.
		if err := os.Chmod(p, 0o600); err != nil {
			return Profile{}, err
		}
	}
	effective := id
	if name != "" {
		effective = name
	}
	return Profile{ID: id, Name: effective, Description: description}, nil
}

// Remove deletes a profile. When it was the active one, the marker falls
// back to the first remaining profile by name, or to a recreated "default"
// when none remain.
func Remove(root, id string) error {
	if err := checkProfileDir(root, id); err != nil {
		return err
	}
	wasActive, err := ActiveID(root)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(root, id)); err != nil {
		return err
	}
	if wasActive != id {
		return nil
	}
	remaining, err := List(root)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		if err := os.MkdirAll(filepath.Join(root, defaultProfileID), 0o755); err != nil {
			return err
		}
		return SetActive(root, defaultProfileID)
	}
	return SetActive(root, remaining[0].ID)
}

// Copy copies the source profile to a new profile with an auto-generated
// id. name is required and must not duplicate an existing effective name
// (including the source's). description, when set, overrides the source's
// description; otherwise the source's description is carried over. The
// destination config is the source's merged config rewritten as a single
// yolo.jsonc (comment loss, same as SaveGlobal; mode 0600 — the config may
// carry provider API keys).
func Copy(root, srcID, name, description string) (Profile, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return Profile{}, errors.New("config: profile name must not be empty")
	}
	if err := checkProfileDir(root, srcID); err != nil {
		return Profile{}, err
	}
	src, err := LoadGlobal(filepath.Join(root, srcID))
	if err != nil {
		return Profile{}, err
	}
	if err := checkNameFree(root, name); err != nil {
		return Profile{}, err
	}
	desc := description
	if desc == "" && src.Profile != nil {
		desc = src.Profile.Description
	}
	id, err := uniqueID(root)
	if err != nil {
		return Profile{}, err
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Profile{}, err
	}
	dst := *src
	dst.Profile = &protocol.Profile{Name: name, Description: desc}
	b, err := json.MarshalIndent(dst, "", "  ")
	if err != nil {
		return Profile{}, err
	}
	p := filepath.Join(dir, yoloFileJSONC)
	if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
		return Profile{}, err
	}
	// 0600: the config may carry provider API keys; the explicit Chmod
	// also upgrades a pre-existing 0644 file.
	if err := os.Chmod(p, 0o600); err != nil {
		return Profile{}, err
	}
	return Profile{ID: id, Name: name, Description: desc}, nil
}

// Edit changes the display name and/or description of an existing profile.
// hasName/hasDesc report which flag was given (absent != empty: an empty
// value clears the field). Renaming to one's own current effective name is
// a successful no-op; a name matching another profile's effective name is
// ErrNameTaken. When the resulting element has both fields empty, the
// "profile" element is dropped from the config. The config is rewritten as
// a single yolo.jsonc (same write pattern as Copy, mode 0600 — the config
// may carry provider API keys); the id (directory name) and the active
// marker are never touched.
func Edit(root, id, name, description string, hasName, hasDesc bool) (Profile, error) {
	if err := checkProfileDir(root, id); err != nil {
		return Profile{}, err
	}
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if hasName && name != "" {
		if err := checkNameFreeFor(root, name, id); err != nil {
			return Profile{}, err
		}
	}
	cfg, err := LoadGlobal(filepath.Join(root, id))
	if err != nil {
		return Profile{}, err
	}
	prof := &protocol.Profile{}
	if cfg.Profile != nil {
		*prof = *cfg.Profile
	}
	if hasName {
		prof.Name = name
	}
	if hasDesc {
		prof.Description = description
	}
	if prof.Name == "" && prof.Description == "" {
		cfg.Profile = nil
	} else {
		cfg.Profile = prof
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return Profile{}, err
	}
	p := filepath.Join(root, id, yoloFileJSONC)
	if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
		return Profile{}, err
	}
	// 0600: the config may carry provider API keys; the explicit Chmod
	// also upgrades a pre-existing 0644 file.
	if err := os.Chmod(p, 0o600); err != nil {
		return Profile{}, err
	}
	effective := id
	if prof.Name != "" {
		effective = prof.Name
	}
	return Profile{ID: id, Name: effective, Description: prof.Description}, nil
}

// checkProfileDir reports ErrNotFound (wrapped) when <root>/<id> is not a
// profile directory.
func checkProfileDir(root, id string) error {
	st, err := os.Stat(filepath.Join(root, id))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%w: %s is not a profile", ErrNotFound, id)
	}
	return nil
}

// checkNameFree reports ErrNameTaken (wrapped) when name (non-empty) equals
// an existing profile's effective name.
func checkNameFree(root, name string) error {
	return checkNameFreeFor(root, name, "")
}

// checkNameFreeFor is checkNameFree with the profile selfID skipped (a
// rename to one's own effective name is a no-op, not a collision).
func checkNameFreeFor(root, name, selfID string) error {
	if name == "" {
		return nil
	}
	profiles, err := List(root)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.ID == selfID {
			continue
		}
		if p.Name == name {
			return fmt.Errorf("%w: %s", ErrNameTaken, name)
		}
	}
	return nil
}

// uniqueID allocates a fresh id that no existing entry (file or dir)
// occupies; it retries a few times before failing.
func uniqueID(root string) (string, error) {
	for range 10 {
		id, err := GenerateID()
		if err != nil {
			return "", err
		}
		if _, err := os.Lstat(filepath.Join(root, id)); errors.Is(err, os.ErrNotExist) {
			return id, nil
		}
	}
	return "", errors.New("config: could not allocate a unique profile id")
}
