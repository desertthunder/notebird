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
	attachment, err := s.storeAttachmentBlob(ctx, filename, contentType, src)
	if err != nil {
		return Attachment{}, err
	}
	if _, err := s.db.ExecContext(ctx, `insert or replace into chirp_attachments (chirp_id, attachment_hash, filename, created_at) values (?, ?, ?, ?)`, chirpID, attachment.Hash, attachment.Filename, attachment.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return Attachment{}, err
	}
	return attachment, nil
}

// StoreDraftAttachment writes a file and associates it with a draft upload token.
func (s *Store) StoreDraftAttachment(ctx context.Context, draftID, filename, contentType string, src io.Reader) (Attachment, error) {
	if strings.TrimSpace(draftID) == "" {
		return Attachment{}, errors.New("draft id is required")
	}
	attachment, err := s.storeAttachmentBlob(ctx, filename, contentType, src)
	if err != nil {
		return Attachment{}, err
	}
	if _, err := s.db.ExecContext(ctx, `insert or replace into attachment_drafts (draft_id, attachment_hash, filename, created_at) values (?, ?, ?, ?)`, draftID, attachment.Hash, attachment.Filename, attachment.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return Attachment{}, err
	}
	return attachment, nil
}

// PromoteDraftAttachments links draft uploads to a newly-created Chirp.
func (s *Store) PromoteDraftAttachments(ctx context.Context, draftID, chirpID string) error {
	if strings.TrimSpace(draftID) == "" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		insert or replace into chirp_attachments (chirp_id, attachment_hash, filename, created_at)
		select ?, attachment_hash, filename, created_at from attachment_drafts where draft_id = ?
	`, chirpID, draftID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from attachment_drafts where draft_id = ?`, draftID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListDraftAttachments returns files linked to a draft upload token.
func (s *Store) ListDraftAttachments(ctx context.Context, draftID string) ([]Attachment, error) {
	rows, err := s.db.QueryContext(ctx, `
		select a.hash, d.filename, a.content_type, a.size, d.created_at
		from attachment_drafts d
		join attachments a on a.hash = d.attachment_hash
		where d.draft_id = ?
		order by d.created_at, d.filename
	`, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAttachments(rows)
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
	return scanAttachments(rows)
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

func (s *Store) storeAttachmentBlob(ctx context.Context, filename, contentType string, src io.Reader) (Attachment, error) {
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
	if _, err := s.db.ExecContext(ctx, `insert or ignore into attachments (hash, size, content_type, created_at) values (?, ?, ?, ?)`, hash, size, contentType, createdAt.Format(time.RFC3339Nano)); err != nil {
		return Attachment{}, err
	}
	return newAttachment(hash, filename, contentType, size, createdAt), nil
}

func scanAttachments(rows interface {
	Scan(dest ...any) error
	Next() bool
	Err() error
}) ([]Attachment, error) {
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
