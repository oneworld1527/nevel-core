package pebble

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotFound = errors.New("not found")
var Sync = &WriteOptions{}

type Options struct{}
type WriteOptions struct{}
type DB struct{ path string }
type Closer struct{}

func Open(path string, _ *Options) (*DB, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, err
	}
	return &DB{path: path}, nil
}
func (d *DB) Close() error { return nil }
func (d *DB) Set(key, value []byte, _ *WriteOptions) error {
	p := d.file(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, value, 0o600)
}
func (d *DB) Get(key []byte) ([]byte, *Closer, error) {
	b, err := os.ReadFile(d.file(key))
	if os.IsNotExist(err) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return b, &Closer{}, nil
}
func (d *DB) Delete(key []byte, _ *WriteOptions) error {
	err := os.Remove(d.file(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (d *DB) NewBatch() *Batch       { return &Batch{db: d} }
func (c *Closer) Close() error       { return nil }
func (d *DB) file(key []byte) string { return filepath.Join(d.path, filepath.Clean(string(key))) }

type op struct {
	del        bool
	key, value []byte
}
type Batch struct {
	db  *DB
	ops []op
}

func (b *Batch) Set(key, value []byte, _ *WriteOptions) error {
	b.ops = append(b.ops, op{key: append([]byte(nil), key...), value: append([]byte(nil), value...)})
	return nil
}
func (b *Batch) Delete(key []byte, _ *WriteOptions) error {
	b.ops = append(b.ops, op{del: true, key: append([]byte(nil), key...)})
	return nil
}
func (b *Batch) Commit(_ *WriteOptions) error {
	for _, op := range b.ops {
		var err error
		if op.del {
			err = b.db.Delete(op.key, Sync)
		} else {
			err = b.db.Set(op.key, op.value, Sync)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (b *Batch) Close() error { return nil }

func (d *DB) WalkPrefix(prefix []byte, fn func(key, value []byte) error) error {
	root := d.file(prefix)
	base := d.path
	return filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasPrefix(path, root) {
			return nil
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		return fn([]byte(filepath.ToSlash(rel)), value)
	})
}
