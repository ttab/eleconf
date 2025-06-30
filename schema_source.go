package eleconf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"text/template"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/ttab/elephantine"
)

type SchemaSource interface {
	LoadSchema(ctx context.Context, name string) (LoadedSchema, error)
}

func NewHttpSchemaSource(
	cx context.Context,
	urlTemplate string, version string,
) (*HttpSchemaSource, error) {
	urlTpl, err := template.New("url").Parse(urlTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse URL template: %w", err)
	}

	return &HttpSchemaSource{
		tpl:     urlTpl,
		version: version,
	}, nil
}

var _ SchemaSource = &HttpSchemaSource{}

type HttpSchemaSource struct {
	tpl     *template.Template
	version string
}

// LoadSchema implements SchemaSource.
func (h *HttpSchemaSource) LoadSchema(
	ctx context.Context, name string,
) (_ LoadedSchema, outErr error) {
	var buf bytes.Buffer

	lock := SchemaLock{
		Name:    name,
		Version: h.version,
	}

	err := h.tpl.Execute(&buf, lock)
	if err != nil {
		return LoadedSchema{}, fmt.Errorf(
			"execute URL template: %w", err)
	}

	lock.URL = buf.String()

	res, err := http.Get(lock.URL)
	if err != nil {
		return LoadedSchema{}, fmt.Errorf(
			"fetch %s: %w", lock.URL, err)
	}

	defer elephantine.Close("schema response body", res.Body, &outErr)

	buf.Reset()

	hash := sha256.New()

	w := io.MultiWriter(&buf, hash)

	_, err = io.Copy(w, res.Body)
	if err != nil {
		return LoadedSchema{}, fmt.Errorf(
			"download %s: %w", lock.URL, err)
	}

	lock.Hash = fmt.Sprintf("%x", hash.Sum(nil))

	schema := LoadedSchema{
		Lock: lock,
		Data: buf.Bytes(),
	}

	return schema, nil
}

func NewGitSchemaSource(
	ctx context.Context, origin string, version string,
) (*GitSchemaSource, error) {
	repo, err := git.CloneContext(
		ctx, memory.NewStorage(), nil,
		&git.CloneOptions{
			URL:           origin,
			ReferenceName: plumbing.ReferenceName(version),
			Depth:         1,
		})
	if err != nil {
		return nil, fmt.Errorf("clone repository: %w", err)
	}

	return &GitSchemaSource{
		repo:    repo,
		version: version,
	}, nil
}

var _ SchemaSource = &GitSchemaSource{}

type GitSchemaSource struct {
	repo    *git.Repository
	version string
}

// LoadSchema implements SchemaSource.
func (g *GitSchemaSource) LoadSchema(ctx context.Context, name string) (LoadedSchema, error) {
	var z LoadedSchema

	ref, err := g.repo.ResolveRevision(plumbing.Revision(g.version))
	if err != nil {
		return z, fmt.Errorf("get current head: %w", err)
	}

	commit, err := g.repo.CommitObject(*ref)
	if err != nil {
		return z, fmt.Errorf("get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return z, fmt.Errorf("get commit tree: %w", err)
	}

	fileRef, err := tree.File(name + ".json")
	if err != nil {
		return z, fmt.Errorf("get file reference: %w", err)
	}

	file, err := fileRef.Reader()
	if err != nil {
		return z, fmt.Errorf("open file: %w", err)
	}

	hash := sha256.New()

	tee := io.TeeReader(file, hash)

	data, err := io.ReadAll(tee)
	if err != nil {
		return z, fmt.Errorf("read file: %w", err)
	}

	return LoadedSchema{
		Lock: SchemaLock{
			Name:    name,
			Hash:    fmt.Sprintf("%x", hash.Sum(nil)),
			Version: g.version,
		},
		Data: data,
	}, nil
}
