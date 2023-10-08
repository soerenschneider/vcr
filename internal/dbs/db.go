package dbs

import (
	"errors"
	"vcr/internal/config"
)

var ErrNotFound = errors.New("not found")

type Db interface {
	Add(programming config.Programming) error
	Delete(name string) error
	Find(name string) (*config.Programming, error)
	List() ([]config.Programming, error)
}

func NewMemoryDb() *MemoryDb {
	return &MemoryDb{db: map[string]config.Programming{}}
}

type MemoryDb struct {
	db map[string]config.Programming
}

func (d *MemoryDb) Add(programming config.Programming) error {
	d.db[programming.Name] = programming
	return nil
}

func (d *MemoryDb) Delete(name string) error {
	delete(d.db, name)
	return nil
}

func (d *MemoryDb) Find(name string) (*config.Programming, error) {
	p, ok := d.db[name]
	if !ok {
		return nil, ErrNotFound
	}
	return &p, nil
}

func (d *MemoryDb) List() ([]config.Programming, error) {
	var ret []config.Programming
	for _, p := range d.db {
		ret = append(ret, p)
	}
	return ret, nil
}
