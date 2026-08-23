// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package version

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// downloadTimeout covers the whole transfer rather than one read, because a
// stalled download that never finishes is the failure worth reporting.
const downloadTimeout = 30 * time.Minute

// download writes the release archive to path, keeping Status in step so the
// web UI can draw a progress bar.
func download(release *Release, path string) error {
	req, err := http.NewRequest("GET", release.AssetUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "casbin-gateway/"+Current().Version)

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s answered HTTP %d", release.AssetName, resp.StatusCode)
	}

	total := release.AssetSize
	if total <= 0 {
		total = resp.ContentLength
	}

	statusLock.Lock()
	status.Total = total
	statusLock.Unlock()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := io.Copy(file, io.TeeReader(resp.Body, &progressWriter{total: total}))
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	// A short file is a download that was cut off. Catching it here turns a
	// broken installation into a message the reader can retry from.
	if release.AssetSize > 0 && written != release.AssetSize {
		return fmt.Errorf("downloaded %d of %d bytes of %s", written, release.AssetSize, release.AssetName)
	}

	return nil
}

// progressWriter counts bytes on their way to disk and publishes the count.
type progressWriter struct {
	total   int64
	written int64
}

func (w *progressWriter) Write(chunk []byte) (int, error) {
	w.written += int64(len(chunk))

	statusLock.Lock()
	status.Downloaded = w.written
	if w.total > 0 {
		status.Percent = int(w.written * 100 / w.total)
		if status.Percent > 100 {
			status.Percent = 100
		}
	}
	statusLock.Unlock()

	return len(chunk), nil
}

// extractBinary writes the one executable inside the archive to path. Only that
// entry is taken, by name, so nothing else in the archive can land on disk.
func extractBinary(archive string, path string) error {
	wanted := filepath.Base(path)

	if strings.HasSuffix(archive, ".zip") {
		return extractFromZip(archive, wanted, path)
	}

	return extractFromTarGz(archive, wanted, path)
}

func extractFromTarGz(archive string, wanted string, path string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("%s is not a gzip archive: %w", filepath.Base(archive), err)
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != wanted {
			continue
		}

		return writeBinary(path, reader)
	}

	return fmt.Errorf("%s holds no %s", filepath.Base(archive), wanted)
}

func extractFromZip(archive string, wanted string, path string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("%s is not a zip archive: %w", filepath.Base(archive), err)
	}
	defer reader.Close()

	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != wanted {
			continue
		}

		content, err := entry.Open()
		if err != nil {
			return err
		}
		defer content.Close()

		return writeBinary(path, content)
	}

	return fmt.Errorf("%s holds no %s", filepath.Base(archive), wanted)
}

func writeBinary(path string, content io.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(content, maxBinarySize+1))
	if err != nil {
		return err
	}
	if written > maxBinarySize {
		return fmt.Errorf("the executable in the archive is larger than %d bytes", int64(maxBinarySize))
	}
	if written == 0 {
		return fmt.Errorf("the executable in the archive is empty")
	}

	return file.Close()
}
