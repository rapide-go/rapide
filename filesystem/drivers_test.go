package storage_test

import (
	"testing"

	storage "github.com/rapide-go/rapide/filesystem"
)

func TestLocalRootConfinement(t *testing.T) {
	disk := storage.NewLocal(t.TempDir())

	if err := disk.Put("avatars/ada.txt", []byte("hello")); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if !disk.Exists("avatars/ada.txt") {
		t.Fatal("expected file to exist")
	}
	data, err := disk.Get("avatars/ada.txt")
	if err != nil || string(data) != "hello" {
		t.Fatalf("unexpected Get: %q (err=%v)", string(data), err)
	}
	if err := disk.Put("../escape.txt", []byte("nope")); err == nil {
		t.Fatal("expected traversal error")
	}
	if err := disk.Delete("avatars/ada.txt"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if disk.Exists("avatars/ada.txt") {
		t.Fatal("expected file to be deleted")
	}
}
