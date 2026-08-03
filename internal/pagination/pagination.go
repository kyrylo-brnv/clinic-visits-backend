package pagination

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	DefaultPageNumber = 1
	DefaultPageSize   = 20
	MaxPageSize       = 200
	PageQueryParam    = "page"
	PerPageQueryParam = "per_page"
)

type Params struct {
	page     int32
	per_page int32
}

func Parse(urlValues url.Values) (Params, error) {
	var params Params
	var err error

	if urlValues.Has(PageQueryParam) {
		params.page, err = parsePositiveInt32(PageQueryParam, urlValues.Get(PageQueryParam))
		if err != nil {
			return Params{}, err
		}
	}

	if urlValues.Has(PerPageQueryParam) {
		params.per_page, err = parsePositiveInt32(PerPageQueryParam, urlValues.Get(PerPageQueryParam))
		if err != nil {
			return Params{}, err
		}

		if params.per_page > MaxPageSize {
			return Params{}, fmt.Errorf(
				"%s must be less than or equal to %d",
				PerPageQueryParam,
				MaxPageSize,
			)
		}
	}

	return params, nil
}

func parsePositiveInt32(name, value string) (int32, error) {
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf(
			"%s must be a positive integer",
			name,
		)
	}

	return int32(parsed), nil
}

func (p *Params) Page() int32 {
	if p.page == 0 {
		return DefaultPageNumber
	}

	return p.page
}

func (p *Params) PerPage() int32 {
	if p.per_page == 0 {
		return DefaultPageSize
	}

	return p.per_page
}

func (p *Params) Limit() int32 {
	return p.PerPage()
}

func (p Params) Offset() int64 {
	return int64(p.Page()-1) * int64(p.PerPage())
}
