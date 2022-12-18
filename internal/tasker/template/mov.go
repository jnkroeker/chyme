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
	if timeout := os.Getenv("CH_TEMPLATE_MOV_TIMEOUT"); timeout != "" {

	}
}

var movTimeout = time.Duration(48) * time.Hour

var Mov = &tasker.Template{
	Name: "Mov",
	Create: func(resource *core.Resource) *core.Task {
		if strings.ToLower(path.Ext(resource.Url.Path)) != ".mov" {
			return nil
		}

		outUrl := *resource.Url
		outUrl.Path = path.Join(os.Getenv("CH_TEMPLATE_MOV_MIRROR_PREFIX"), outUrl.Host, outUrl.Path) + "/"
		outUrl.Host = os.Getenv("CH_TEMPLATE_MOV_MIRROR_BUCKET")

		return &core.Task{
			InputResource:    resource,
			OutputResource:   &core.Resource{Url: &outUrl},
			MetadataResource: defaultMetadataResource(),
			Hooks:            "mov",
			ExecutionStrategy: &core.ExecutionStrategy{
				Executor: "docker",
				Config: map[string]string{
					"image": "jnkroeker/mov_converter:0.7",
				},
			},
			Timeout: mie4nitfTimeout,
		}
	},
}
