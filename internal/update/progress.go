package update

import (
	"context"
	"io"

	"github.com/creativeprojects/go-selfupdate"
)

// progressSource wraps a GitHubSource to report download progress.
type progressSource struct {
	*selfupdate.GitHubSource
	onProgress ProgressFunc
}

// DownloadReleaseAsset wraps the underlying source's download with
// progress tracking.
func (s *progressSource) DownloadReleaseAsset(ctx context.Context, rel *selfupdate.Release, assetID int64) (io.ReadCloser, error) {
	rc, err := s.GitHubSource.DownloadReleaseAsset(ctx, rel, assetID)
	if err != nil {
		return nil, err
	}
	if s.onProgress == nil {
		return rc, nil
	}
	return &progressReadCloser{
		rc:         rc,
		total:      int64(rel.AssetByteSize),
		onProgress: s.onProgress,
	}, nil
}

// progressReadCloser wraps an io.ReadCloser and reports bytes read via
// a callback.
type progressReadCloser struct {
	rc         io.ReadCloser
	downloaded int64
	total      int64
	onProgress ProgressFunc
}

func (r *progressReadCloser) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if n > 0 {
		r.downloaded += int64(n)
		r.onProgress(r.downloaded, r.total)
	}
	return n, err
}

func (r *progressReadCloser) Close() error {
	return r.rc.Close()
}

// newUpdaterWithProgress creates an updater that reports download
// progress via the given callback. If onProgress is nil, it behaves
// identically to newUpdater.
func newUpdaterWithProgress(onProgress ProgressFunc) (*selfupdate.Updater, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, err
	}

	var src selfupdate.Source = source
	if onProgress != nil {
		src = &progressSource{
			GitHubSource: source,
			onProgress:   onProgress,
		}
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    src,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
		OS:        osLabel(),
		Arch:      archLabel(),
	})
	if err != nil {
		return nil, err
	}
	return updater, nil
}
