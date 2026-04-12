package eleconf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ttab/newsdoc"
)

// ExemplarLock is a locked exemplar entry in the lockfile.
type ExemplarLock struct {
	DocType string `json:"doc_type"`
	Name    string `json:"name"`
	Hash    string `json:"hash"`
}

// LoadedExemplar is an exemplar document loaded from disk with its canonical
// form and hash.
type LoadedExemplar struct {
	Lock      ExemplarLock
	Document  newsdoc.Document
	Canonical []byte
}

// LoadExemplars recursively loads all .json files from the exemplars directory.
// The document type is determined from the "type" field in each document, not
// from the directory structure.
func LoadExemplars(dir string) ([]LoadedExemplar, error) {
	exemplarsDir := filepath.Join(dir, "exemplars")

	info, err := os.Stat(exemplarsDir)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat exemplars directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("exemplars path is not a directory: %s",
			exemplarsDir)
	}

	var exemplars []LoadedExemplar

	err = filepath.Walk(exemplarsDir, func(
		path string, info os.FileInfo, walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}

		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		ex, lErr := loadExemplarFile(path, exemplarsDir)
		if lErr != nil {
			relPath, _ := filepath.Rel(exemplarsDir, path)

			return fmt.Errorf("load exemplar %q: %w", relPath, lErr)
		}

		exemplars = append(exemplars, ex)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return exemplars, nil
}

func loadExemplarFile(path string, baseDir string) (LoadedExemplar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LoadedExemplar{}, fmt.Errorf("read file: %w", err)
	}

	var doc newsdoc.Document

	err = json.Unmarshal(data, &doc)
	if err != nil {
		return LoadedExemplar{}, fmt.Errorf("parse document: %w", err)
	}

	if doc.Type == "" {
		return LoadedExemplar{}, fmt.Errorf("document has no type field")
	}

	// Canonicalize by re-marshalling through encoding/json (sorts keys).
	canonical, err := json.Marshal(doc)
	if err != nil {
		return LoadedExemplar{}, fmt.Errorf("canonicalize document: %w", err)
	}

	h := sha256.Sum256(canonical)
	hash := "sha256:" + hex.EncodeToString(h[:])

	if doc.URI == "" {
		return LoadedExemplar{}, fmt.Errorf("document has no uri field")
	}

	name := doc.URI

	return LoadedExemplar{
		Lock: ExemplarLock{
			DocType: doc.Type,
			Name:    name,
			Hash:    hash,
		},
		Document:  doc,
		Canonical: canonical,
	}, nil
}

// ExemplarLocks extracts the lock entries from loaded exemplars.
func ExemplarLocks(exemplars []LoadedExemplar) []ExemplarLock {
	locks := make([]ExemplarLock, len(exemplars))
	for i, ex := range exemplars {
		locks[i] = ex.Lock
	}

	return locks
}
