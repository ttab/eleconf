package eleconf

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func NewSchemaCache() (*SchemaCache, error) {
	userCache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("get user cache dir: %w", err)
	}

	cacheDir := filepath.Join(userCache, "eleconf", "schema_cache")

	err = os.MkdirAll(cacheDir, 0o700)
	if err != nil {
		return nil, fmt.Errorf("create schema cache dir: %w", err)
	}

	return &SchemaCache{
		cacheDir: cacheDir,
	}, nil
}

type SchemaCache struct {
	cacheDir string
}

type CacheEntry struct {
	URL  string
	Hash string
}

func (sc *SchemaCache) Read(
	assetURL string, logicalURL string, hash string,
) ([]byte, bool, error) {
	path, err := urlToPath(sc.cacheDir, logicalURL)
	if err != nil {
		return nil, false, err
	}

	infoPath := path + ".info"

	infoData, err := os.ReadFile(infoPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("read info file: %w", err)
	}

	var ce CacheEntry

	err = json.Unmarshal(infoData, &ce)
	if err != nil {
		return nil, false, fmt.Errorf("invalid info file: %w", err)
	}

	if ce.Hash != hash {
		return nil, false, fmt.Errorf(
			"hash mismatch: got %s expected %s",
			ce.Hash, hash)
	}

	schemaData, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read schema file: %w", err)
	}

	sum := fmt.Sprintf("%x", sha256.Sum256(schemaData))
	if sum != ce.Hash {
		return nil, false, fmt.Errorf("schema file didn't match the stored or requested hash")
	}

	return schemaData, true, nil
}

func urlToPath(base string, logicalURL string) (string, error) {
	pu, err := url.Parse(logicalURL)
	if err != nil {
		return "", fmt.Errorf("invalid asset URL: %w", err)
	}

	subpath := []string{
		base,
		pu.Scheme,
		pu.Hostname(),
	}

	uPath := strings.Split(pu.Path, "/")

	subpath = append(subpath, uPath...)

	return filepath.Join(subpath...), nil
}

func (sc *SchemaCache) Store(
	assetURL string, logicalURL string, hash string, data []byte,
) error {
	path, err := urlToPath(sc.cacheDir, logicalURL)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	err = os.MkdirAll(dir, 0o700)
	if err != nil {
		return fmt.Errorf("create entry directory: %w", err)
	}

	info := CacheEntry{
		URL:  assetURL,
		Hash: hash,
	}

	infoPath := path + ".info"

	infoData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal info JSON: %w", err)
	}

	err = os.WriteFile(infoPath, infoData, 0o600)
	if err != nil {
		return fmt.Errorf("write info file: %w", err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return fmt.Errorf("write schema file: %w", err)
	}

	return nil
}
