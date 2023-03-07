/*
Copyright 2022 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package time contains utilities for working with times, dates, and durations.
package time

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var pattern = regexp.MustCompile(`^(R(?P<repetition>\d+)/)?P((?P<year>\d+)Y)?((?P<month>\d+)M)?((?P<week>\d+)W)?((?P<day>\d+)D)?(T((?P<hour>\d+)H)?((?P<minute>\d+)M)?((?P<second>\d+)S)?)?$`)

// ParseISO8601Duration parses a duration from a string in the ISO8601 duration format.
func ParseISO8601Duration(from string) (int, int, int, time.Duration, int, error) {
	match := pattern.FindStringSubmatch(from)
	if match == nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("unsupported ISO8601 duration format %q", from)
	}
	years, months, days, duration := 0, 0, 0, time.Duration(0)
	// -1 signifies infinite repetition
	repetition := -1
	for i, name := range pattern.SubexpNames() {
		part := match[i]
		if i == 0 || name == "" || part == "" {
			continue
		}
		val, err := strconv.Atoi(part)
		if err != nil {
			return 0, 0, 0, 0, 0, err
		}
		switch name {
		case "year":
			years = val
		case "month":
			months = val
		case "week":
			days += 7 * val
		case "day":
			days += val
		case "hour":
			duration += time.Hour * time.Duration(val)
		case "minute":
			duration += time.Minute * time.Duration(val)
		case "second":
			duration += time.Second * time.Duration(val)
		case "repetition":
			repetition = val
		default:
			return 0, 0, 0, 0, 0, fmt.Errorf("unsupported ISO8601 duration field %s", name)
		}
	}
	return years, months, days, duration, repetition, nil
}

func ParseISO8601Duration_New(from string) (years int, months int, days int, duration time.Duration, repetition int, err error) {
	// -1 signifies infinite repetition
	repetition = -1

	// Length must be at least 2 characters per specs
	l := len(from)
	if l < 2 {
		err = errors.New("unsupported ISO8601 duration format: " + from)
		return
	}

	var i int

	// Check if the first character is "R", indicating we have repetitions
	if from[0] == 'R' {
		// Scan until the "/" character to get the repetitions
		for {
			i++
			if i == l || from[i] == '/' {
				break
			}
		}

		if i-1 < 1 {
			err = errors.New("unsupported ISO8601 duration format: " + from)
			return
		}
		repetition, err = strconv.Atoi(from[1:i])
		if err != nil {
			err = errors.New("unsupported ISO8601 duration format: " + from)
			return
		}

		i++

		// If we're already at the end of the string after getting repetitions, return
		if i >= l {
			return
		}
	}

	// First character must be a "P"
	if from[i] != 'P' {
		err = errors.New("unsupported ISO8601 duration format: " + from)
		return
	}
	i++

	start := i
	isParsingTime := false
	var tmp int
	for i < l {
		switch from[i] {
		case 'T':
			if start != i {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			isParsingTime = true
			start = i + 1

		case 'Y':
			if isParsingTime || start == i {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			years, err = strconv.Atoi(from[start:i])
			if err != nil {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			start = i + 1

		case 'W':
			if isParsingTime || start == i {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			tmp, err = strconv.Atoi(from[start:i])
			if err != nil {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			days += tmp * 7
			start = i + 1

		case 'D':
			if isParsingTime || start == i {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			tmp, err = strconv.Atoi(from[start:i])
			if err != nil {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			days += tmp
			start = i + 1

		case 'H':
			if !isParsingTime || start == i {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			tmp, err = strconv.Atoi(from[start:i])
			if err != nil {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			duration += time.Duration(tmp) * time.Hour
			start = i + 1

		case 'S':
			if !isParsingTime || start == i {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			tmp, err = strconv.Atoi(from[start:i])
			if err != nil {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			duration += time.Duration(tmp) * time.Second
			start = i + 1

		case 'M': // "M" can be used for both months and minutes
			if start == i {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			tmp, err = strconv.Atoi(from[start:i])
			if err != nil {
				err = errors.New("unsupported ISO8601 duration format: " + from)
				return
			}
			if isParsingTime {
				duration += time.Duration(tmp) * time.Minute
			} else {
				months = tmp
			}
			start = i + 1
		}

		i++
	}

	return
}

// ParseDuration creates time.Duration from either:
// - ISO8601 duration format
// - time.Duration string format
func ParseDuration(from string) (int, int, int, time.Duration, int, error) {
	y, m, d, dur, r, err := ParseISO8601Duration_New(from)
	if err == nil {
		return y, m, d, dur, r, nil
	}
	dur, err = time.ParseDuration(from)
	if err == nil {
		return 0, 0, 0, dur, -1, nil
	}
	return 0, 0, 0, 0, 0, fmt.Errorf("unsupported duration format %q", from)
}

// ParseTime creates time.Duration from either:
// - ISO8601 duration format
// - time.Duration string format
// - RFC3339 datetime format
// For duration formats, an offset is added.
func ParseTime(from string, offset *time.Time) (time.Time, error) {
	var start time.Time
	if offset != nil {
		start = *offset
	} else {
		start = time.Now()
	}
	y, m, d, dur, r, err := ParseISO8601Duration(from)
	if err == nil {
		if r != -1 {
			return time.Time{}, errors.New("repetitions are not allowed")
		}
		return start.AddDate(y, m, d).Add(dur), nil
	}
	if dur, err = time.ParseDuration(from); err == nil {
		return start.Add(dur), nil
	}
	if t, err := time.Parse(time.RFC3339, from); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time/duration format %q", from)
}
