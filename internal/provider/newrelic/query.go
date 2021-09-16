// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package newrelic

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	limitClause = " limit "
	bitSize     = 64
)

// Query stores user configured query for external metric and allows extending by limits or filters.
type Query string

func (q Query) addLimit() Query {
	if strings.Contains(strings.ToLower(string(q)), limitClause) {
		return q
	}

	return Query(fmt.Sprintf("%s limit 1", q))
}

func (q Query) addClusterFilter(clusterName string, addClusterFilter bool) Query {
	if !addClusterFilter {
		return q
	}

	return Query(fmt.Sprintf("%s where clusterName='%s'", q, clusterName))
}

func (q Query) addMatchFilter(match labels.Selector) Query {
	if match == nil {
		return q
	}

	requirements, ok := match.Requirements()
	if !ok || len(requirements) == 0 {
		return q
	}

	whereClause := "where"

	for index, r := range requirements {
		key := r.Key()

		switch r.Operator() {
		case selection.Equals, selection.DoubleEquals, selection.GreaterThan, selection.LessThan, selection.NotEquals:
			whereClause = buildSimpleCondition(whereClause, key, r.Operator(), r.Values().List()[0])

		case selection.In, selection.NotIn:
			whereClause = buildINClause(whereClause, key, r.Operator(), r.Values().List())

		case selection.DoesNotExist, selection.Exists:
			whereClause = fmt.Sprintf("%s %s %s", whereClause, key, transformOperator(r.Operator()))
		}

		if index != len(requirements)-1 {
			whereClause = fmt.Sprintf("%s and", whereClause)
		}
	}

	return Query(fmt.Sprintf("%s %s", q, whereClause))
}

func buildINClause(whereClause string, key string, operator selection.Operator, values []string) string {
	inClause := "("

	for index, v := range values {
		if _, errNoNumber := strconv.ParseFloat(v, bitSize); errNoNumber != nil {
			inClause = fmt.Sprintf("%s'%s'", inClause, v)
		} else {
			inClause = fmt.Sprintf("%s%s", inClause, v)
		}

		if index != len(values)-1 {
			inClause = fmt.Sprintf("%s, ", inClause)
		}
	}

	inClause = fmt.Sprintf("%s)", inClause)

	return fmt.Sprintf("%s %s %s %s", whereClause, key, transformOperator(operator), inClause)
}

func buildSimpleCondition(whereClause string, key string, operator selection.Operator, value string) string {
	// Note that this is a simplification since it is possible that we have a valid number, but we want it as a string.
	// Es: systemMemoryBytes is a number and reported as a string
	if _, errNoNumber := strconv.ParseFloat(value, bitSize); errNoNumber != nil {
		return fmt.Sprintf("%s %s %s '%s'", whereClause, key, transformOperator(operator), value)
	}

	return fmt.Sprintf("%s %s %s %s", whereClause, key, transformOperator(operator), value)
}

func transformOperator(op selection.Operator) string {
	m := map[selection.Operator]string{
		selection.NotEquals:    "!=",
		selection.Equals:       "=",
		selection.GreaterThan:  ">",
		selection.DoubleEquals: "=",
		selection.LessThan:     ",",
		selection.Exists:       "IS NOT NULL",
		selection.DoesNotExist: "IS NULL",
		selection.In:           "IN",
		selection.NotIn:        "NOT IN",
	}

	return m[op]
}
