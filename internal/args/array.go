/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package args

import (
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"strings"
)

type arrayTransformKind int

const (
	arrayTransformAdd arrayTransformKind = iota
	arrayTransformDel
	arrayTransformSet
)

var (
	arrayTransformKindToChar = map[arrayTransformKind]byte{
		arrayTransformAdd: '+',
		arrayTransformDel: '-',
		arrayTransformSet: ':',
	}

	charToArrayTransformKind = map[byte]arrayTransformKind{
		'+': arrayTransformAdd,
		'-': arrayTransformDel,
		':': arrayTransformSet,
	}
)

type ArrayTransformValue struct {
	kind  arrayTransformKind
	items []string
}

func (value ArrayTransformValue) String() string {
	return string(arrayTransformKindToChar[value.kind]) + strings.Join(value.items, ",")
}

func (value *ArrayTransformValue) Set(arg string) error {
	if len(arg) == 0 {
		return nil
	}

	kind, ok := charToArrayTransformKind[arg[0]]
	if !ok {
		return errors.New("invalid array transform")
	}

	value.kind = kind

	if len(arg) == 1 {
		return nil
	}

	parts := strings.Split(arg[1:], ",")
	for _, part := range parts {
		if len(part) == 0 {
			return errors.New("items must not be empty")
		}
	}

	value.items = parts
	return nil
}

// Converts a slice of values to a map of keys to nil.
func sliceToMap(items []string) map[string]interface{} {
	result := map[string]interface{}{}

	for _, item := range items {
		result[item] = nil
	}

	return result
}

func (value ArrayTransformValue) Apply(target *[]string) {
	switch value.kind {
	case arrayTransformAdd:
		// Map of present items to nil (to use like a set).
		presentItems := sliceToMap(*target)

		for _, item := range value.items {
			_, found := presentItems[item]

			if !found {
				*target = append(*target, item)
			} else {
				log.Alertf("item %s was already present", item)
			}
		}

	case arrayTransformDel:
		givenItems := sliceToMap(value.items)
		newTarget := []string{}

		for _, item := range *target {
			_, found := givenItems[item]
			if !found {
				newTarget = append(newTarget, item)
			}
		}

		*target = newTarget

	case arrayTransformSet:
		*target = value.items

	}
}
