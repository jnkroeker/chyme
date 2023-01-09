package ingest

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"kroekerlabs.dev/chyme/services/internal/core"
)

/** Filters **/

type FilterFunc = func(resource *core.Resource) *core.Resource

type ResourceFilter struct {
	Description string
	Factory     func(args []string) (FilterFunc, error)
}

var FilterRegistry = map[string]ResourceFilter{
	"identity": {"Applies no filter.", NewIdentityFilter},
	"ext":      {"Filters by file extension. Example: ext/txt", NewExtFilter},
}

func NewExtFilter(args []string) (FilterFunc, error) {
	// regex here looks for '.' specificly and supplants %s with args[0]
	// our extension will accept anything before the '.'
	re, err := regexp.Compile(fmt.Sprintf(`^(.+)\.%s$`, args[0]))
	if err != nil {
		return nil, fmt.Errorf("extension regexp failed to compile: %s", err.Error())
	}

	return func(resource *core.Resource) *core.Resource {
		// b, err := resource.MarshalBinary()
		// if err != nil {
		// 	return "error converting url to binary", err
		// }
		if re.Match([]byte(strings.ToLower(resource.Url.String()))) {
			return resource
		}
		return nil
	}, nil
}

func NewIdentityFilter(_ []string) (FilterFunc, error) {
	return func(resource *core.Resource) *core.Resource {
		return resource
	}, nil
}

func NewFilter(filterString string) (FilterFunc, error) {
	split := strings.Split(filterString, "/")
	// use the first part of 'ext/pdf' find the right filter function in the map
	// the FilterRegistry maps a filter type (like extension) to functions to be performed
	filter, ok := FilterRegistry[split[0]]
	if !ok {
		return nil, fmt.Errorf("invalid filter %s", split[0])
	}
	return filter.Factory(split[1:])
}

var ErrPatternMatch = errors.New("No match")
