package geo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// Default configuration values for the geo database.
const (
	DefaultRefreshInterval = 24 * time.Hour
	DefaultDBPath          = "/tmp/geolite2-country.mmdb"

	// MaxMind download endpoint:
	// https://dev.maxmind.com/geoip/updating-databases/#directly-downloading-databases
	// Authentication: HTTP Basic Auth - account ID as username, license key as password.
	downloadURL        = "https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz"
	downloadTimeout    = 5 * time.Minute
	maxDownloadRetries = 3
	retryBaseDelay     = 5 * time.Second
)

// Database is the interface for geo DB operations used by the server.
type Database interface {
	IsLoaded() bool
	Lookup(ipStr string) (*Data, error)
}

// DB manages the MaxMind GeoLite2-Country reader with hot-swap support.
type DB struct {
	mu     sync.RWMutex
	reader *geoip2.Reader
}

// Reader returns the current active geoip2 reader under a read lock.
// The caller must invoke the returned unlock function when done.
func (d *DB) Reader() (*geoip2.Reader, func()) {
	d.mu.RLock()
	return d.reader, d.mu.RUnlock
}

// IsLoaded reports whether the MaxMind DB has been successfully loaded.
func (d *DB) IsLoaded() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.reader != nil
}

// Start loads or downloads the MaxMind DB and launches the background refresh
// goroutine. It calls log.Fatalf if no usable DB can be obtained on startup.
func (d *DB) Start(dbPath, accountID, licenseKey string, interval time.Duration) {
	firstRefreshIn := d.loadOrDownload(dbPath, accountID, licenseKey, interval)
	go d.refreshLoop(dbPath, accountID, licenseKey, firstRefreshIn, interval)
}

// loadOrDownload checks if a usable .mmdb file already exists at dbPath and
// its modification time is still within interval. If so it loads the file
// directly without any network request and returns the remaining time until
// the next refresh. Otherwise it downloads a fresh copy and returns interval.
func (d *DB) loadOrDownload(dbPath, accountID, licenseKey string, interval time.Duration) time.Duration {
	if info, err := os.Stat(dbPath); err == nil {
		age := time.Since(info.ModTime())
		if age < interval {
			if loadErr := d.openAndSwap(dbPath); loadErr == nil {
				remaining := interval - age
				log.Printf("gogeoip: loaded existing MaxMind DB (age %s, next refresh in %s)",
					age.Truncate(time.Second), remaining.Truncate(time.Second))
				return remaining
			}
			log.Printf("gogeoip: existing DB unreadable, downloading fresh copy")
		} else {
			log.Printf("gogeoip: existing DB is stale (age %s >= interval %s), downloading fresh copy",
				age.Truncate(time.Second), interval)
		}
	}
	if err := d.downloadAndLoad(dbPath, accountID, licenseKey); err != nil {
		log.Fatalf("gogeoip: failed to load MaxMind GeoLite2-Country database: %v", err)
	}
	return interval
}

// refreshLoop waits firstRefreshIn before the first download (aligned with
// when the DB file was last written), then repeats every interval.
func (d *DB) refreshLoop(dbPath, accountID, licenseKey string, firstRefreshIn, interval time.Duration) {
	timer := time.NewTimer(firstRefreshIn)
	defer timer.Stop()
	<-timer.C

	d.runRefresh(dbPath, accountID, licenseKey)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		d.runRefresh(dbPath, accountID, licenseKey)
	}
}

func (d *DB) runRefresh(dbPath, accountID, licenseKey string) {
	log.Println("gogeoip: starting scheduled MaxMind DB refresh")
	if err := d.downloadAndLoad(dbPath, accountID, licenseKey); err != nil {
		log.Printf("gogeoip: DB refresh failed (will retry next cycle): %v", err)
	} else {
		log.Println("gogeoip: MaxMind DB refresh completed successfully")
	}
}

// downloadAndLoad downloads the GeoLite2-Country .mmdb, extracts it to dbPath,
// opens it, and hot-swaps the reader. Retries up to maxDownloadRetries times.
func (d *DB) downloadAndLoad(dbPath, accountID, licenseKey string) error {
	var lastErr error
	for attempt := 1; attempt <= maxDownloadRetries; attempt++ {
		if attempt > 1 {
			delay := retryBaseDelay * time.Duration(attempt-1)
			log.Printf("gogeoip: download attempt %d/%d (waiting %v)...", attempt, maxDownloadRetries, delay)
			time.Sleep(delay)
		}
		if err := d.tryDownloadAndLoad(dbPath, accountID, licenseKey); err != nil {
			lastErr = err
			log.Printf("gogeoip: attempt %d failed: %v", attempt, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("all %d download attempts failed; last error: %w", maxDownloadRetries, lastErr)
}

func (d *DB) tryDownloadAndLoad(dbPath, accountID, licenseKey string) error {
	tmpPath := dbPath + ".tmp"

	if err := downloadGeoLite2(accountID, licenseKey, tmpPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	reader, err := geoip2.Open(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("open mmdb: %w", err)
	}

	// Hot-swap: replace old reader under write lock.
	d.mu.Lock()
	old := d.reader
	d.reader = reader
	d.mu.Unlock()

	// Atomic replace on POSIX.
	if err := os.Rename(tmpPath, dbPath); err != nil {
		log.Printf("gogeoip: WARNING - could not rename temp DB file: %v", err)
		_ = os.Remove(tmpPath)
	}

	if old != nil {
		_ = old.Close()
	}

	log.Println("gogeoip: MaxMind DB loaded successfully")
	return nil
}

// openAndSwap opens an existing .mmdb at path and hot-swaps the reader.
func (d *DB) openAndSwap(path string) error {
	reader, err := geoip2.Open(path)
	if err != nil {
		return fmt.Errorf("open mmdb %s: %w", path, err)
	}
	d.mu.Lock()
	old := d.reader
	d.reader = reader
	d.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// downloadGeoLite2 fetches the GeoLite2-Country tar.gz from MaxMind.
func downloadGeoLite2(accountID, licenseKey, destPath string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(accountID, licenseKey)

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MaxMind download returned HTTP %d", resp.StatusCode)
	}

	return extractMMDBFromTarGz(resp.Body, destPath)
}

// extractMMDBFromTarGz reads a .tar.gz stream and extracts the first .mmdb.
func extractMMDBFromTarGz(r io.Reader, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		if filepath.Ext(hdr.Name) != ".mmdb" {
			continue
		}
		return writeFile(tr, destPath)
	}
	return errors.New("no .mmdb file found in MaxMind tar.gz archive")
}

func writeFile(r io.Reader, path string) error {
	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}
