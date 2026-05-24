package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestStoreAttachment(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	chirp, err := store.CreateChirp(ctx, "With attachment", "body")
	if err != nil {
		t.Fatal(err)
	}

	body := "hello attachment"
	attachment, err := store.StoreAttachment(ctx, chirp.ID, "../hello.txt", "text/plain", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	wantHashBytes := sha256.Sum256([]byte(body))
	wantHash := hex.EncodeToString(wantHashBytes[:])
	if attachment.Hash != wantHash {
		t.Fatalf("hash = %q, want %q", attachment.Hash, wantHash)
	}
	if attachment.Filename != "hello.txt" {
		t.Fatalf("filename = %q, want sanitized filename", attachment.Filename)
	}

	path, contentType, err := store.AttachmentFile(ctx, attachment.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "text/plain" {
		t.Fatalf("content type = %q", contentType)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != body {
		t.Fatalf("stored body = %q", stored)
	}

	loaded, err := store.GetChirp(ctx, chirp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Attachments) != 1 || loaded.Attachments[0].Hash != attachment.Hash {
		t.Fatalf("unexpected attachments: %#v", loaded.Attachments)
	}
}
