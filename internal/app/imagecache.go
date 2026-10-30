package app

import (
	"fmt"

	"my-weread-desk/internal/weread"
)

const imageCacheVersion = "2"

// repairImageCache drops chapters decoded by an older parser that could not
// resolve images. It only removes chapters whose blocks still contain an
// unresolved image, so text-only chapters and cached media are kept.
func (a *App) repairImageCache() {
	if a.store.GetMeta("imageCacheVersion") == imageCacheVersion {
		return
	}
	n, err := a.store.DropChaptersWithMissingImages()
	if err != nil {
		a.logf("清理旧图片缓存失败：%v", err)
		return
	}
	if n > 0 {
		a.logf("已清理 %d 个缺图章节缓存，重新打开章节会重新解析", n)
	}
	_ = a.store.SetMeta("imageCacheVersion", imageCacheVersion)
}

func hasMissingImage(blocks []weread.Block) bool {
	for _, block := range blocks {
		if block.Kind == "image" && block.Src == "" {
			return true
		}
	}
	return false
}

// shouldRetryImages allows one repair fetch per chapter per app session so a
// chapter that genuinely has no images does not refetch on every open.
func (a *App) shouldRetryImages(bookID string, uid int64) bool {
	key := fmt.Sprintf("%s:%d", bookID, uid)
	a.imageMu.Lock()
	defer a.imageMu.Unlock()
	if a.imageRetries[key] {
		return false
	}
	a.imageRetries[key] = true
	return true
}
