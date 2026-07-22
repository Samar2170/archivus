package visionsvc

import "archivus/internal/store"

type VisionService struct {
	store *store.Store
}

func (vs *VisionService) MarkImagesInFileMetadatas() error {
	return nil
}
