package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCheckBulkPublication(t *testing.T) {
	full := make([]string, MaxBulkPublication)
	for i := range full {
		full[i] = fmt.Sprintf("nd-%04d", i)
	}
	for _, c := range []struct {
		name string
		ids  []string
		want error
	}{
		{"empty", nil, ErrBulkPublicationEmpty},
		{"exactly the cap", full, nil},
		{"one past the cap", append(append([]string{}, full...), "nd-extra"), ErrBulkPublicationTooLarge},
		{"missing id", []string{"nd-1", ""}, ErrBulkPublicationMissingID},
		{"duplicate id", []string{"nd-1", "nd-2", "nd-1"}, ErrBulkPublicationDuplicateID},
		{"id too long", []string{strings.Repeat("x", 65)}, ErrIDThamChieuQuaDai},
		{"ordinary", []string{"nd-1", "nd-2"}, nil},
	} {
		err := CheckBulkPublication(c.ids)
		if c.want == nil && err != nil {
			t.Errorf("%s: err = %v, want nil", c.name, err)
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}
