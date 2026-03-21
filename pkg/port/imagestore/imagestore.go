package imagestore

import "github.com/hiparker/echo-fade-memory/pkg/core/model"

// Store persists image metadata and lightweight links.
type Store interface {
	Upsert(asset *model.ImageAsset) error
	Get(id string) (*model.ImageAsset, error)
	GetBySHA256(sha256 string) (*model.ImageAsset, error)
	Delete(id string) error
	Find(query string, limit int) ([]*model.ImageAsset, error)
	ListAll() ([]*model.ImageAsset, error)
	ListRecent(limit int) ([]*model.ImageAsset, error)

	ReplaceLinks(imageID string, links []model.ImageLink) error
	ListLinks(imageID string) ([]model.ImageLink, error)
	ListLinksByTarget(linkType, targetID string, limit int) ([]model.ImageLink, error)

	CountAssets() (int, error)
	CountAssetsWithCaption() (int, error)
	CountAssetsWithOCR() (int, error)
	CountLinks() (int, error)

	Close() error
}
