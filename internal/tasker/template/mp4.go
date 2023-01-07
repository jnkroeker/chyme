package template

import (
	"os"
	"path"
	"strings"
	"time"

	"kroekerlabs.dev/chyme/services/internal/core"
	"kroekerlabs.dev/chyme/services/internal/tasker"
)

func init() {
	if timeout := os.Getenv("CH_TEMPLATE_MP4_TIMEOUT"); timeout != "" {

	}
}

var mp4Timeout = time.Duration(48) * time.Hour

var Mp4 = &tasker.Template{
	Name: "Mp4",
	Create: func(resource *core.Resource) *core.Task {
		if strings.ToLower(path.Ext(resource.Url.Path)) != ".mp4" {
			return nil
		}

		outUrl := *resource.Url
		outUrl.Path = path.Join(os.Getenv("CH_TEMPLATE_MP4_MIRROR_PREFIX"), outUrl.Host, outUrl.Path) + "/"
		outUrl.Host = os.Getenv("CH_TEMPLATE_MP4_MIRROR_BUCKET")

		return &core.Task{
			InputResource:    resource,
			OutputResource:   &core.Resource{Url: &outUrl},
			MetadataResource: defaultMetadataResource(),
			Hooks:            "mp4",
			ExecutionStrategy: &core.ExecutionStrategy{
				Executor: "docker",
				Config: map[string]string{
					"image": "jnkroeker/mp4_processor:0.1.0",
				},
			},
			Timeout: mie4nitfTimeout,
		}
	},
}
