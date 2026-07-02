// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package webpush

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestSubscriptionColumnsMatchStruct guards the explicit column list against
// drift: if someone adds a db-tagged field to Subscription but forgets to
// update subscriptionColumns (or vice versa), the SELECTs would either miss a
// column or fail. Keeping them in lockstep is what makes dropping `SELECT *`
// safe.
func TestSubscriptionColumnsMatchStruct(t *testing.T) {
	var tags []string
	tp := reflect.TypeOf(Subscription{})
	for i := 0; i < tp.NumField(); i++ {
		if tag := tp.Field(i).Tag.Get("db"); tag != "" && tag != "-" {
			tags = append(tags, tag)
		}
	}

	cols := strings.Split(subscriptionColumns, ",")
	for i := range cols {
		cols[i] = strings.TrimSpace(cols[i])
	}

	sort.Strings(tags)
	sort.Strings(cols)
	if !reflect.DeepEqual(tags, cols) {
		t.Errorf("subscriptionColumns %v do not match Subscription db tags %v", cols, tags)
	}
}
