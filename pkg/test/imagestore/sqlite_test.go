package imagestore_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	imgsqlite "github.com/hiparker/echo-fade-memory/pkg/port/imagestore/sqlite"
)

func TestSQLiteImageStoreUpsertQueryAndCounts(t *testing.T) {
	store, err := imgsqlite.New(filepath.Join(t.TempDir(), "images.db"))
	if err != nil {
		t.Fatalf("imgsqlite.New: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	now := time.Now()
	asset := &model.ImageAsset{
		ID:            "img-1",
		FilePath:      "/tmp/cat-shot.png",
		SHA256:        "sha-1",
		SourceSession: "session:1",
		SourceKind:    "chat_image",
		CreatedAt:     now,
		UpdatedAt:     now,
		Caption:       "cat screenshot image",
		Tags:          []string{"cat", "screenshot"},
		OCRText:       "HELLO WORLD",
	}
	if err := store.Upsert(asset); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.ReplaceLinks("img-1", []model.ImageLink{
		{ImageID: "img-1", LinkType: "memory", TargetID: "mem-1", CreatedAt: now},
	}); err != nil {
		t.Fatalf("ReplaceLinks: %v", err)
	}

	got, err := store.Get("img-1")
	if err != nil || got == nil {
		t.Fatalf("Get: asset=%+v err=%v", got, err)
	}
	if got.Caption != "cat screenshot image" {
		t.Fatalf("caption = %q", got.Caption)
	}

	byHash, err := store.GetBySHA256("sha-1")
	if err != nil || byHash == nil || byHash.ID != "img-1" {
		t.Fatalf("GetBySHA256: asset=%+v err=%v", byHash, err)
	}

	found, err := store.Find("HELLO", 10)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) != 1 || found[0].ID != "img-1" {
		t.Fatalf("unexpected find results: %+v", found)
	}

	recent, err := store.ListRecent(10)
	if err != nil || len(recent) != 1 {
		t.Fatalf("ListRecent: %+v err=%v", recent, err)
	}

	links, err := store.ListLinks("img-1")
	if err != nil || len(links) != 1 {
		t.Fatalf("ListLinks: %+v err=%v", links, err)
	}

	targetLinks, err := store.ListLinksByTarget("memory", "mem-1", 10)
	if err != nil || len(targetLinks) != 1 {
		t.Fatalf("ListLinksByTarget: %+v err=%v", targetLinks, err)
	}

	allAssets, err := store.ListAll()
	if err != nil || len(allAssets) != 1 {
		t.Fatalf("ListAll: %+v err=%v", allAssets, err)
	}

	totalAssets, err := store.CountAssets()
	if err != nil || totalAssets != 1 {
		t.Fatalf("CountAssets: count=%d err=%v", totalAssets, err)
	}
	captioned, err := store.CountAssetsWithCaption()
	if err != nil || captioned != 1 {
		t.Fatalf("CountAssetsWithCaption: count=%d err=%v", captioned, err)
	}
	ocrCount, err := store.CountAssetsWithOCR()
	if err != nil || ocrCount != 1 {
		t.Fatalf("CountAssetsWithOCR: count=%d err=%v", ocrCount, err)
	}
	linkCount, err := store.CountLinks()
	if err != nil || linkCount != 1 {
		t.Fatalf("CountLinks: count=%d err=%v", linkCount, err)
	}

	if err := store.Delete("img-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	remaining, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected empty assets after delete, got %+v", remaining)
	}
}
