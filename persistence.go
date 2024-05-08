/*
reference: https://eli.thegreenplace.net/2020/implementing-raft-part-3-persistence-and-optimizations/
*/
package raft

import (
	"bytes"
	"encoding/gob"
	"log"

	"github.com/boltdb/bolt"
)

const (
	currentTermKey = "currentTerm"
	votedForKey    = "votedFor"
	logKey         = "log"

	bucketName = "raft"
)

// Persistence is an interface implemented by stable storage providers.
type Persistence interface {
	Set(key string, value []byte)
	Get(key string) ([]byte, bool)
	HasData() bool
}

// BoltDBStorage is an implementation of the Persistence interface using BoltDB.
type BoltDBStorage struct {
	db *bolt.DB
}

// NewBoltDBStorage creates a new BoltDBStorage instance.
func NewBoltDBStorage(dbPath string) (*BoltDBStorage, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &BoltDBStorage{db: db}, nil
}

// Set writes a key-value pair to the database.
func (s *BoltDBStorage) Set(key string, value []byte) {
	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), value)
	})
	if err != nil {
		log.Fatal(err)
	}
}

// Get retrieves a value for a given key from the database.
func (s *BoltDBStorage) Get(key string) ([]byte, bool) {
	var value []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		value = b.Get([]byte(key))
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return value, value != nil
}

// HasData checks if there is any data in the persistence.
func (s *BoltDBStorage) HasData() bool {
	if v, found := s.Get(currentTermKey); found {
		return v != nil
	}
	if v, found := s.Get(votedForKey); found {
		return v != nil
	}
	if v, found := s.Get(logKey); found {
		return v != nil
	}
	return false
}

func (n *Node) restoreFromStorage() {
	if termData, found := n.persistence.Get(currentTermKey); found {
		d := gob.NewDecoder(bytes.NewBuffer(termData))
		if err := d.Decode(&n.currentTerm); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("currentTerm not found in storage")
	}
	if votedData, found := n.persistence.Get(votedForKey); found {
		d := gob.NewDecoder(bytes.NewBuffer(votedData))
		if err := d.Decode(&n.votedFor); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("votedFor not found in storage")
	}
	if logData, found := n.persistence.Get(logKey); found {
		d := gob.NewDecoder(bytes.NewBuffer(logData))
		if err := d.Decode(&n.log); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("log not found in storage")
	}
}

func (n *Node) persist() {
	var termData bytes.Buffer
	if err := gob.NewEncoder(&termData).Encode(n.currentTerm); err != nil {
		log.Fatal(err)
	}
	n.persistence.Set(currentTermKey, termData.Bytes())

	var votedData bytes.Buffer
	if err := gob.NewEncoder(&votedData).Encode(n.votedFor); err != nil {
		log.Fatal(err)
	}
	n.persistence.Set(votedForKey, votedData.Bytes())

	var logData bytes.Buffer
	if err := gob.NewEncoder(&logData).Encode(n.log); err != nil {
		log.Fatal(err)
	}
	n.persistence.Set(logKey, logData.Bytes())
}
