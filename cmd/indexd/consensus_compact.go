package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const (
	consensusCompactMinSize         int64   = 4 << 30  // 4 GiB
	consensusCompactMinReclaim      int64   = 1 << 30  // 1 GiB
	consensusCompactMinReclaimRatio float64 = 0.10     // 10%
	consensusCompactTxMaxSize       int64   = 64 << 20 // 64 MiB
)

func consensusDBPath(dir string) string {
	return filepath.Join(dir, "consensus.db")
}

func consensusCompactTempPath(path string) string {
	return path + ".compact"
}

func consensusCompactBackupPath(path string) string {
	return path + ".precompact"
}

func recoverConsensusCompaction(path string) error {
	backup := consensusCompactBackupPath(path)
	temp := consensusCompactTempPath(path)

	_, pathErr := os.Stat(path)
	_, backupErr := os.Stat(backup)

	switch {
	case pathErr == nil && backupErr == nil:
		// A previous compaction completed the swap but was interrupted before
		// the old database could be removed.
		if err := os.Remove(backup); err != nil {
			return fmt.Errorf("failed to remove stale consensus backup: %w", err)
		}
	case errors.Is(pathErr, os.ErrNotExist) && backupErr == nil:
		// A previous compaction was interrupted between moving the original
		// database aside and installing the compacted copy. Restore the original.
		if err := os.Rename(backup, path); err != nil {
			return fmt.Errorf("failed to restore consensus backup: %w", err)
		}
	case pathErr != nil && !errors.Is(pathErr, os.ErrNotExist):
		return fmt.Errorf("failed to stat consensus database: %w", pathErr)
	case backupErr != nil && !errors.Is(backupErr, os.ErrNotExist):
		return fmt.Errorf("failed to stat consensus backup: %w", backupErr)
	}

	if err := os.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove stale compacted consensus database: %w", err)
	}
	return nil
}

func shouldCompactConsensusDB(size, reclaimable int64) bool {
	return size >= consensusCompactMinSize &&
		reclaimable >= consensusCompactMinReclaim &&
		float64(reclaimable)/float64(size) >= consensusCompactMinReclaimRatio
}

func consensusReclaimableBytes(path string) (int64, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		ReadOnly:        true,
		PreLoadFreelist: true,
	})
	if err != nil {
		return 0, err
	}
	defer db.Close()

	stats := db.Stats()
	pages := int64(stats.FreePageN + stats.PendingPageN)
	return pages * int64(db.Info().PageSize), nil
}

func checkConsensusDB(path string) error {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		ReadOnly:        true,
		PreLoadFreelist: true,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	return db.View(func(tx *bolt.Tx) error {
		var firstErr error
		for err := range tx.Check() {
			if firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	})
}

func compactConsensusDBIfNeeded(path string, log *zap.Logger) (compacted bool, before, after, reclaimable int64, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, 0, 0, 0, err
	}
	before = info.Size()
	if before < consensusCompactMinSize {
		return false, before, before, 0, nil
	}

	reclaimable, err = consensusReclaimableBytes(path)
	if err != nil {
		return false, before, before, 0, fmt.Errorf("failed to inspect consensus database: %w", err)
	} else if !shouldCompactConsensusDB(before, reclaimable) {
		return false, before, before, reclaimable, nil
	}

	log.Info("compacting consensus database",
		zap.Int64("sizeBytes", before),
		zap.Int64("reclaimableBytes", reclaimable))

	temp := consensusCompactTempPath(path)
	backup := consensusCompactBackupPath(path)
	if err := os.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, before, before, reclaimable, fmt.Errorf("failed to remove stale compact file: %w", err)
	}
	if err := os.Remove(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, before, before, reclaimable, fmt.Errorf("failed to remove stale backup file: %w", err)
	}

	src, err := bolt.Open(path, 0600, &bolt.Options{ReadOnly: true})
	if err != nil {
		return false, before, before, reclaimable, fmt.Errorf("failed to open consensus database for compaction: %w", err)
	}

	dst, err := bolt.Open(temp, info.Mode().Perm(), nil)
	if err != nil {
		_ = src.Close()
		return false, before, before, reclaimable, fmt.Errorf("failed to create compacted consensus database: %w", err)
	}

	compactErr := bolt.Compact(dst, src, consensusCompactTxMaxSize)
	closeDstErr := dst.Close()
	closeSrcErr := src.Close()
	if compactErr != nil || closeDstErr != nil || closeSrcErr != nil {
		_ = os.Remove(temp)
		return false, before, before, reclaimable, errors.Join(compactErr, closeDstErr, closeSrcErr)
	}
	if err := os.Chmod(temp, info.Mode().Perm()); err != nil {
		_ = os.Remove(temp)
		return false, before, before, reclaimable, fmt.Errorf("failed to preserve consensus database permissions: %w", err)
	}
	if err := checkConsensusDB(temp); err != nil {
		_ = os.Remove(temp)
		return false, before, before, reclaimable, fmt.Errorf("compacted consensus database failed integrity check: %w", err)
	}

	compactInfo, err := os.Stat(temp)
	if err != nil {
		_ = os.Remove(temp)
		return false, before, before, reclaimable, fmt.Errorf("failed to stat compacted consensus database: %w", err)
	}
	after = compactInfo.Size()
	if after >= before {
		_ = os.Remove(temp)
		return false, before, before, reclaimable, nil
	}

	if err := os.Rename(path, backup); err != nil {
		_ = os.Remove(temp)
		return false, before, before, reclaimable, fmt.Errorf("failed to move original consensus database aside: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Rename(backup, path)
		_ = os.Remove(temp)
		return false, before, before, reclaimable, fmt.Errorf("failed to install compacted consensus database: %w", err)
	}
	if err := checkConsensusDB(path); err != nil {
		_ = os.Remove(path)
		_ = os.Rename(backup, path)
		return false, before, before, reclaimable, fmt.Errorf("installed compacted consensus database failed integrity check: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return true, before, after, reclaimable, fmt.Errorf("compaction succeeded but failed to remove old consensus database: %w", err)
	}
	return true, before, after, reclaimable, nil
}
