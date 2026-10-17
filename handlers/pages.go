package handlers

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// maxPageSelectors bounds how many page entries a single request may name.
const maxPageSelectors = 1000

// pageSelector matches a single page or inclusive range in page-selection fields.
var pageSelector = regexp.MustCompile(`^\d{1,6}(-\d{1,6})?$`)

type pageRange struct {
	first int
	last  int
}

func (pages pageRange) selector() string {
	selection := strconv.Itoa(pages.first)
	if pages.last != pages.first {
		selection += "-" + strconv.Itoa(pages.last)
	}
	return selection
}

func parsePageRanges(pages string) ([]pageRange, error) {
	if len(pages) > maxPageSelectors*14 {
		return nil, errors.New("page selection is too long")
	}
	parts := strings.Split(pages, ",")
	if len(parts) > maxPageSelectors {
		return nil, errors.New("too many page ranges")
	}
	ranges := make([]pageRange, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !pageSelector.MatchString(part) {
			return nil, errors.New("invalid page selection: use numbers or ranges, e.g. 1,3,6-7")
		}
		bounds := strings.Split(part, "-")
		first, _ := strconv.Atoi(bounds[0]) // The pattern bounds integers to six digits.
		last := first
		if len(bounds) == 2 {
			last, _ = strconv.Atoi(bounds[1])
		}
		if first < 1 || last < first {
			return nil, errors.New("page ranges must start at 1 or greater and end at or after their start")
		}
		ranges = append(ranges, pageRange{first: first, last: last})
	}
	return ranges, nil
}

func validatePageRanges(ranges []pageRange, pageCount int) error {
	for _, pages := range ranges {
		if pages.last > pageCount {
			return fmt.Errorf("page selection exceeds the PDF's %d pages", pageCount)
		}
	}
	return nil
}
