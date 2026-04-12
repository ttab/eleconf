package eleconf

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// NewSchemaLockFile creates a lockfile from loaded schemas and exemplars.
func NewSchemaLockFile(
	schemas []LoadedSchema, exemplars []ExemplarLock,
) *SchemaLockfile {
	lf := SchemaLockfile{
		Updated:   time.Now(),
		Schemas:   make(map[string]SchemaLock),
		Exemplars: exemplars,
	}

	for _, l := range schemas {
		lf.Schemas[l.Lock.Name] = l.Lock
	}

	return &lf
}

// LoadLockFile reads and parses a lockfile from disk.
func LoadLockFile(fileName string) (*SchemaLockfile, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read lock file: %w", err)
	}

	var lf SchemaLockfile

	err = json.Unmarshal(data, &lf)
	if err != nil {
		return nil, fmt.Errorf("parse lock file: %w", err)
	}

	return &lf, nil
}

// SchemaLockfile tracks the locked versions and hashes of schemas and
// exemplars.
type SchemaLockfile struct {
	Updated   time.Time             `json:"updated"`
	Schemas   map[string]SchemaLock `json:"schemas"`
	Exemplars []ExemplarLock        `json:"exemplars,omitempty"`
}

// Save writes the lockfile to disk.
func (lf *SchemaLockfile) Save(fileName string) error {
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lock data: %w", err)
	}

	err = os.WriteFile(fileName, data, 0o600)
	if err != nil {
		return fmt.Errorf("write to file: %w", err)
	}

	return nil
}

// Check validates that a loaded schema matches the lockfile entry.
func (lf *SchemaLockfile) Check(
	name string, loaded LoadedSchema, init bool,
) error {
	lock, ok := lf.Schemas[name]

	switch {
	case init && !ok:
		return nil
	case !ok:
		return fmt.Errorf("missing lock file entry for %q, run eleconf update", name)
	case init && loaded.Lock.Version != lock.Version:
		return nil
	case loaded.Lock.Version != lock.Version:
		return fmt.Errorf(
			"lock file version mismatch for %q, got %s expected %s, run eleconf update",
			name, loaded.Lock.Version, lock.Version)
	case loaded.Lock.Hash != lock.Hash:
		return fmt.Errorf("lock file hash mismatch for %q", name)
	}

	return nil
}

// CheckExemplars validates that loaded exemplars match the lockfile entries.
func (lf *SchemaLockfile) CheckExemplars(exemplars []LoadedExemplar) error {
	locked := make(map[string]ExemplarLock, len(lf.Exemplars))
	for _, ex := range lf.Exemplars {
		locked[ex.Name] = ex
	}

	for _, ex := range exemplars {
		lock, ok := locked[ex.Lock.Name]
		if !ok {
			return fmt.Errorf(
				"missing lock file entry for exemplar %q, run eleconf update",
				ex.Lock.Name)
		}

		if ex.Lock.Hash != lock.Hash {
			return fmt.Errorf(
				"lock file hash mismatch for exemplar %q, run eleconf update",
				ex.Lock.Name)
		}

		delete(locked, ex.Lock.Name)
	}

	for name := range locked {
		return fmt.Errorf(
			"exemplar %q in lock file but not on disk, run eleconf update",
			name)
	}

	return nil
}
