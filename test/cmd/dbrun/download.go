package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// download fetches a URL to a path.
//
// It writes beside the target and renames, so that an interrupted download
// does not look like a finished one on the next run. Both files this command
// fetches are large enough for that to matter: the SQL Server installer is a
// few hundred megabytes and the Oracle archive is three gigabytes.
func download(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("asking for %s: %w", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s", url, resp.Status)
	}

	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating %s: %w", tmp, err)
	}
	p := &progress{total: resp.ContentLength, last: time.Now()}
	if _, err := io.Copy(io.MultiWriter(f, p), resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming %s: %w", tmp, err)
	}
	return nil
}

// progress says how far a long download has got.
//
// Three gigabytes is minutes, and a command that prints nothing for minutes
// looks like a command that has hung.
type progress struct {
	total int64
	done  int64
	last  time.Time
}

// Write counts, and prints every few seconds rather than every read.
func (p *progress) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if time.Since(p.last) < 5*time.Second {
		return len(b), nil
	}
	p.last = time.Now()
	if p.total > 0 {
		fmt.Printf("  %s of %s\n", megabytes(p.done), megabytes(p.total))
	} else {
		fmt.Printf("  %s\n", megabytes(p.done))
	}
	return len(b), nil
}

// megabytes is enough precision for a progress line and needs no dependency.
func megabytes(n int64) string {
	const mb = 1 << 20
	return fmt.Sprintf("%dMB", n/mb)
}
