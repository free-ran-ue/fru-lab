package context

import (
	"context"
	"fmt"
	"io"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	dockerimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/jsonmessage"
)

// KnownImage describes one of the docker images the app's compose templates
// pin by name - not every image docker happens to have cached locally, just
// the ones this app actually deploys, so the Images page only ever shows
// things the operator recognizes.
type KnownImage struct {
	// Key is the stable identifier used in API routes (e.g. "amf") - image
	// refs themselves contain '/' and ':', which don't survive as a single
	// gin path param.
	Key   string
	Name  string
	Ref   string
	Group string
}

// knownImages mirrors the `image:` lines across templates/free5gc,
// templates/gnb and templates/ue.docker-compose.yaml.tmpl exactly - gNB and
// UE both deploy from the same free-ran-ue image, so it only appears once
// here under its own group.
var knownImages = []KnownImage{
	{Key: "mongo", Name: "MongoDB", Ref: "mongo:4.4", Group: "free5gc"},
	{Key: "amf", Name: "AMF", Ref: "free5gc/amf:latest", Group: "free5gc"},
	{Key: "ausf", Name: "AUSF", Ref: "free5gc/ausf:latest", Group: "free5gc"},
	{Key: "chf", Name: "CHF", Ref: "free5gc/chf:latest", Group: "free5gc"},
	{Key: "nrf", Name: "NRF", Ref: "free5gc/nrf:latest", Group: "free5gc"},
	{Key: "nssf", Name: "NSSF", Ref: "free5gc/nssf:latest", Group: "free5gc"},
	{Key: "pcf", Name: "PCF", Ref: "free5gc/pcf:latest", Group: "free5gc"},
	{Key: "smf", Name: "SMF", Ref: "free5gc/smf:latest", Group: "free5gc"},
	{Key: "udm", Name: "UDM", Ref: "free5gc/udm:latest", Group: "free5gc"},
	{Key: "udr", Name: "UDR", Ref: "free5gc/udr:latest", Group: "free5gc"},
	{Key: "upf", Name: "UPF", Ref: "free5gc/upf:latest", Group: "free5gc"},
	{Key: "webui", Name: "WebUI", Ref: "free5gc/webui:latest", Group: "free5gc"},
	{Key: "free-ran-ue", Name: "free-ran-ue (gNB / UE)", Ref: "alonza0314/free-ran-ue:latest", Group: "free-ran-ue"},
}

// FindKnownImage looks up a known image by its route key, nil if unknown.
func FindKnownImage(key string) *KnownImage {
	for i := range knownImages {
		if knownImages[i].Key == key {
			return &knownImages[i]
		}
	}
	return nil
}

// ImageInfo is a KnownImage joined with whatever the local docker daemon
// currently knows about it.
type ImageInfo struct {
	KnownImage
	Present   bool
	Size      int64
	CreatedAt *time.Time
}

// ListImages reports, for every image this app deploys, whether it's
// present in the local docker image cache right now, and its size/age if so.
func (c *composeContext) ListImages(ctx context.Context) ([]ImageInfo, error) {
	apiClient := c.dockerCli.Client()

	infos := make([]ImageInfo, 0, len(knownImages))
	for _, known := range knownImages {
		info := ImageInfo{KnownImage: known}

		inspect, err := apiClient.ImageInspect(ctx, known.Ref)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return nil, fmt.Errorf("failed to inspect image %s: %v", known.Ref, err)
			}
			infos = append(infos, info)
			continue
		}

		info.Present = true
		info.Size = inspect.Size
		if created, err := time.Parse(time.RFC3339Nano, inspect.Created); err == nil {
			info.CreatedAt = &created
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// RemoveImage clears a known image out of the local docker image cache
// (docker's own "image is in use" error surfaces as-is if a container still
// references it) so the next deploy is forced to pull it fresh.
func (c *composeContext) RemoveImage(ctx context.Context, key string) error {
	known := FindKnownImage(key)
	if known == nil {
		return fmt.Errorf("unknown image key %s", key)
	}

	if _, err := c.dockerCli.Client().ImageRemove(ctx, known.Ref, dockerimage.RemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove image %s: %v", known.Ref, err)
	}

	return nil
}

// PullImage re-pulls a known image's pinned tag (always "latest" or a fixed
// version, per the templates) from its registry. The pull response is a
// streamed sequence of progress messages that must be fully drained for the
// pull to actually run to completion; jsonmessage also surfaces any
// mid-stream error (e.g. manifest not found) that a plain HTTP status can't.
func (c *composeContext) PullImage(ctx context.Context, key string) error {
	known := FindKnownImage(key)
	if known == nil {
		return fmt.Errorf("unknown image key %s", key)
	}

	rc, err := c.dockerCli.Client().ImagePull(ctx, known.Ref, dockerimage.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %v", known.Ref, err)
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			c.CtxLog.Warnf("Failed to close pull stream for image %s: %v", known.Ref, closeErr)
		}
	}()

	if err := jsonmessage.DisplayJSONMessagesStream(rc, io.Discard, 0, false, nil); err != nil {
		return fmt.Errorf("failed to pull image %s: %v", known.Ref, err)
	}

	return nil
}
