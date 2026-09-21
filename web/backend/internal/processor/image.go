package processor

import (
	flctx "backend/internal/context"
	"backend/model"
	"context"
	"net/http"
)

func (p *Processor) ImageList(ctx context.Context) (*model.ResponseImageList, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing image list")

	results, err := p.FlContext.ListImages(ctx)
	if err != nil {
		p.ProcLog.Errorf("Failed to list images: %v", err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to list images",
		}
	}

	images := make([]model.ImageInfo, 0, len(results))
	for _, result := range results {
		images = append(images, model.ImageInfo{
			Key:       result.Key,
			Name:      result.Name,
			Image:     result.Ref,
			Group:     result.Group,
			Present:   result.Present,
			Size:      result.Size,
			CreatedAt: result.CreatedAt,
		})
	}

	return &model.ResponseImageList{
		Images: images,
	}, nil
}

func (p *Processor) ImageRemove(ctx context.Context, key string) (*model.ResponseImageAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing image remove for key: %s", key)

	if flctx.FindKnownImage(key) == nil {
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusNotFound,
			Detail:     "Unknown image " + key,
		}
	}

	if err := p.FlContext.RemoveImage(ctx, key); err != nil {
		p.ProcLog.Errorf("Failed to remove image %s: %v", key, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to clear image " + key,
		}
	}

	return &model.ResponseImageAction{
		Message: "Cleared image " + key,
	}, nil
}

func (p *Processor) ImagePull(ctx context.Context, key string) (*model.ResponseImageAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing image pull for key: %s", key)

	if flctx.FindKnownImage(key) == nil {
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusNotFound,
			Detail:     "Unknown image " + key,
		}
	}

	if err := p.FlContext.PullImage(ctx, key); err != nil {
		p.ProcLog.Errorf("Failed to pull image %s: %v", key, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to pull image " + key,
		}
	}

	return &model.ResponseImageAction{
		Message: "Pulled image " + key,
	}, nil
}
