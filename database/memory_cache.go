package database

import (
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"time"
)

type Cache struct {
	*cache.Cache
}

func NewMemCache() DB {
	c := Cache{cache.New(5*time.Minute, 10*time.Minute)}
	return &c
}

func (db *Cache) Put(key []byte, value []byte) error {
	db.Set(string(key), string(value), cache.DefaultExpiration)
	return nil
}

func (db *Cache) Get(key []byte) ([]byte, error) {
	val, found := db.Cache.Get(string(key))
	if !found {
		return nil, errors.New(fmt.Sprintf("could not find item %v", key))
	}

	return val.([]byte), nil
}

func (db *Cache) Delete(key []byte) error {
	db.Cache.Delete(string(key))
	return nil
}

func (db *Cache) Close() {

}
