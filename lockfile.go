package eleconf

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func NewSchemaLockFile(loaded []LoadedSchema) *SchemaLockfile {
	lf := SchemaLockfile{
		Updated: time.Now(),
		Schemas: make(map[string]SchemaLock),
	}

	for _, l := range loaded {
		lf.Schemas[l.Lock.Name] = l.Lock
	}

	return &lf
}

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

type SchemaLockfile struct {
	Updated time.Time             `json:"updated"`
	Schemas map[string]SchemaLock `json:"schemas"`
}

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

func (lf *SchemaLockfile) Check(
	name string, loaded LoadedSchema, init bool,
) error {
	lock, ok := lf.Schemas[name]

	switch {
	case init && !ok:
		return nil
	case !ok:
		return fmt.Errorf("missing lock file entry for %q, run eleconf init", name)
	case init && loaded.Lock.Version != lock.Version:
		return nil
	case loaded.Lock.Version != lock.Version:
		return fmt.Errorf(
			"lock file version mismatch for %q, got %s expected %s, run eleconf init",
			name, loaded.Lock.Version, lock.Version)
	case loaded.Lock.Hash != lock.Hash:
		return fmt.Errorf("lock file hash mismatch for %q", name)
	}

	return nil
}
