package main

import (
	"encoding/json"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const (
	dbFile     = "jobs.db"
	jobsBucket = "Jobs"
)

var errJobNotFound = errors.New("job not found")

func openDB(path string) (*bolt.DB, error) {
	db, err := bolt.Open(path, 0o644, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(jobsBucket)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketUsageTotals)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketUsageHistory)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init bucket: %w", err)
	}
	return db, nil
}

func putJob(db *bolt.DB, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(jobsBucket))
		return b.Put([]byte(job.JobID), data)
	})
}

func getJob(db *bolt.DB, id string) (*Job, error) {
	var raw []byte
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(jobsBucket))
		if b == nil {
			return fmt.Errorf("bucket %q missing", jobsBucket)
		}
		v := b.Get([]byte(id))
		if v == nil {
			return errJobNotFound
		}
		raw = append([]byte(nil), v...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	return &job, nil
}

// jobStats counts persisted jobs by status and returns total. Corrupt entries are skipped.
func jobStats(db *bolt.DB) (total int, byStatus map[JobStatus]int, err error) {
	byStatus = make(map[JobStatus]int)
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(jobsBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var job Job
			if json.Unmarshal(v, &job) != nil {
				return nil
			}
			total++
			byStatus[job.Status]++
			return nil
		})
	})
	return total, byStatus, err
}
