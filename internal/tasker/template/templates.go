// Package templates contains Task Templates. Task Templates serve as a blueprint to create Tasks that define what
// processing will occur on the Worker cluster.
package template

import (
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"kroekerlabs.dev/chyme/services/internal/core"
	"kroekerlabs.dev/chyme/services/internal/tasker"
)

func init() {
	if timeout := os.Getenv("CH_TEMPLATE_MIE4NITF_TIMEOUT"); timeout != "" {

	}
}

func defaultMetadataResource() *core.Resource {
	return &core.Resource{Url: &url.URL{
		Scheme: "s3",
		Host:   os.Getenv("CH_TEMPLATE_MIE4NITF_LOGGING_BUCKET"),
		Path:   os.Getenv("CH_TEMPLATE_MIE4NITF_LOGGING_PREFIX"),
	}}
}

var mie4nitfTimeout = time.Duration(48) * time.Hour

// template includes the source bucket in the key of the output resource.
var Mie4NitfV2 = &tasker.Template{
	Name: "Wavelet: mie-4-nitf",
	Create: func(resource *core.Resource) *core.Task {
		if strings.ToLower(path.Ext(resource.Url.Path)) != ".nui" {
			return nil 
		}

		// Do not process this if it is a chunked NUI
		// chunk, _ := mie4nitf.ParseChunk(resource.Url.Path)
		// if chunk != nil {
		// 	return nil
		// }

		outUrl := *resource.Url 
		outUrl.Path = path.Join(os.Getenv("CH_TEMPLATE_MIE4NITF_MIRROR_PREFIX"), outUrl.Host, outUrl.Path) + "/"
		outUrl.Host = os.Getenv("CH_TEMPLATE_MIE4NITF_MIRROR_BUCKET")

		return &core.Task{
			InputResource:    resource,
			OutputResource:   &core.Resource{Url: &outUrl},
			MetadataResource: defaultMetadataResource(),
			Hooks:            "mie4nitf",
			ExecutionStrategy: &core.ExecutionStrategy{
				Executor: "docker",
				Config: map[string]string{
					"image": "test", //TODO: WHAT IS THIS? THE WAVELET? => os.Getenv("CH_TEMPLATE_MIE4NITF_IMAGE"),
				},
			},
			Timeout: mie4nitfTimeout,
		}
	},
}