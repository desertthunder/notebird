package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// StoreAttachment writes a file into content-addressed storage and links it to a Chirp.
func (s *Store) StoreAttachment(ctx context.Context, chirpID, filename, contentType string, src io.Reader) (Attachment, error) {
	if strings.TrimSpace(chirpID) == "" {
		return Attachment{}, errors.New("chirp id is required")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `select exists(select 1 from chirps where id = ?)`, chirpID).Scan(&exists); err != nil {
		return Attachment{}, err
	}
	if !exists {
		return Attachment{}, errors.New("chirp not found")
	}
	filename = cleanAttachmentFilename(filename)
	if filename == "" {
		return Attachment{}, errors.New("attachment filename is required")
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	tmpDir := filepath.Join(s.dataDir, "attachments", "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return Attachment{}, err
	}
	tmp, err := os.CreateTemp(tmpDir, "upload-*")
	if err != nil {
		return Attachment{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), src)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Attachment{}, err
	}
	if size == 0 {
		return Attachment{}, errors.New("attachment file is empty")
	}

	hash := hex.EncodeToString(h.Sum(nil))
	path := s.attachmentPath(hash)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Attachment{}, err
		}
		if err := os.Rename(tmpName, path); err != nil {
			return Attachment{}, err
		}
	} else if err != nil {
		return Attachment{}, err
	}

	createdAt := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Attachment{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `insert or ignore into attachments (hash, size, content_type, created_at) values (?, ?, ?, ?)`, hash, size, contentType, createdAt.Format(time.RFC3339Nano)); err != nil {
		return Attachment{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert or replace into chirp_attachments (chirp_id, attachment_hash, filename, created_at) values (?, ?, ?, ?)`, chirpID, hash, filename, createdAt.Format(time.RFC3339Nano)); err != nil {
		return Attachment{}, err
	}
	if err := tx.Commit(); err != nil {
		return Attachment{}, err
	}
	return newAttachment(hash, filename, contentType, size, createdAt), nil
}

// ListAttachments returns files linked to a Chirp in attachment order.
func (s *Store) ListAttachments(ctx context.Context, chirpID string) ([]Attachment, error) {
	rows, err := s.db.QueryContext(ctx, `
		select a.hash, ca.filename, a.content_type, a.size, ca.created_at
		from chirp_attachments ca
		join attachments a on a.hash = ca.attachment_hash
		where ca.chirp_id = ?
		order by ca.created_at, ca.filename
	`, chirpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []Attachment
	for rows.Next() {
		var hash, filename, contentType, created string
		var size int64
		if err := rows.Scan(&hash, &filename, &contentType, &size, &created); err != nil {
			return nil, err
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, created)
		attachments = append(attachments, newAttachment(hash, filename, contentType, size, createdAt))
	}
	return attachments, rows.Err()
}

// AttachmentFile resolves an attachment hash to its on-disk path and content type.
func (s *Store) AttachmentFile(ctx context.Context, hash string) (string, string, error) {
	if !sha256Pattern.MatchString(hash) {
		return "", "", errors.New("invalid attachment hash")
	}
	var contentType string
	if err := s.db.QueryRowContext(ctx, `select content_type from attachments where hash = ?`, hash).Scan(&contentType); err != nil {
		return "", "", err
	}
	return s.attachmentPath(hash), contentType, nil
}

func (s *Store) attachmentPath(hash string) string {
	return filepath.Join(s.dataDir, "attachments", "sha256", hash[:2], hash[2:4], hash)
}

func cleanAttachmentFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\x00", "")
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func newAttachment(hash, filename, contentType string, size int64, createdAt time.Time) Attachment {
	return Attachment{
		Hash:        hash,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		CreatedAt:   createdAt,
		URL:         fmt.Sprintf("/attachments/%s?name=%s", hash, url.QueryEscape(filename)),
		IsImage:     strings.HasPrefix(contentType, "image/"),
	}
}
