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

package time

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	t.Run("parse time.Duration", func(t *testing.T) {
		y, m, d, duration, repetition, err := ParseDuration("0h30m0s")
		require.NoError(t, err)
		assert.Equal(t, time.Minute*30, duration)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, -1, repetition)
	})

	t.Run("parse ISO 8601 duration with repetition", func(t *testing.T) {
		y, m, d, duration, repetition, err := ParseDuration("R5/P10Y5M3DT30M")
		require.NoError(t, err)
		assert.Equal(t, 10, y)
		assert.Equal(t, 5, m)
		assert.Equal(t, 3, d)
		assert.Equal(t, time.Minute*30, duration)
		assert.Equal(t, 5, repetition)
	})

	t.Run("parse ISO 8601 duration without repetition", func(t *testing.T) {
		y, m, d, duration, repetition, err := ParseDuration("P1MT2H10M3S")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 1, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Hour*2+time.Minute*10+time.Second*3, duration)
		assert.Equal(t, -1, repetition)

		y, m, d, duration, repetition, err = ParseDuration("P2W")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 14, d)
		assert.Equal(t, time.Duration(0), duration)
		assert.Equal(t, -1, repetition)

		y, m, d, duration, repetition, err = ParseDuration("PT1S")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Second, duration)
		assert.Equal(t, -1, repetition)

		y, m, d, duration, repetition, err = ParseDuration("P1M")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 1, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Duration(0), duration)
		assert.Equal(t, -1, repetition)

		y, m, d, duration, repetition, err = ParseDuration("PT1M")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Minute, duration)
		assert.Equal(t, -1, repetition)

		y, m, d, duration, repetition, err = ParseDuration("P0D")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Duration(0), duration)
		assert.Equal(t, -1, repetition)

		y, m, d, duration, repetition, err = ParseDuration("PT0S")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Duration(0), duration)
		assert.Equal(t, -1, repetition)

		// This is technically invalid because it's out of order, but we'll accept anyways
		y, m, d, duration, repetition, err = ParseDuration("P1M2D")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 1, m)
		assert.Equal(t, 2, d)
		assert.Equal(t, time.Duration(0), duration)
		assert.Equal(t, -1, repetition)
	})

	t.Run("parse ISO 8601 duration with repetition only", func(t *testing.T) {
		y, m, d, duration, repetition, err := ParseDuration("R5")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Duration(0), duration)
		assert.Equal(t, 5, repetition)

		// With ending slash
		y, m, d, duration, repetition, err = ParseDuration("R5/")
		require.NoError(t, err)
		assert.Equal(t, 0, y)
		assert.Equal(t, 0, m)
		assert.Equal(t, 0, d)
		assert.Equal(t, time.Duration(0), duration)
		assert.Equal(t, 5, repetition)
	})

	t.Run("parse ISO8610 and calculate with leap year", func(t *testing.T) {
		y, m, d, dur, _, err := ParseDuration("P1Y2M3D")
		require.NoError(t, err)

		// 2020 is a leap year
		start, _ := time.Parse("2006-01-02 15:04:05", "2020-02-03 11:12:13")
		target := start.AddDate(y, m, d).Add(dur)
		expect, _ := time.Parse("2006-01-02 15:04:05", "2021-04-06 11:12:13")
		assert.Equal(t, expect, target)

		// 2019 is not a leap year
		start, _ = time.Parse("2006-01-02 15:04:05", "2019-02-03 11:12:13")
		target = start.AddDate(y, m, d).Add(dur)
		expect, _ = time.Parse("2006-01-02 15:04:05", "2020-04-06 11:12:13")
		assert.Equal(t, expect, target)
	})

	t.Run("parse RFC3339 datetime", func(t *testing.T) {
		_, _, _, _, _, err := ParseDuration(time.Now().Add(time.Minute).Format(time.RFC3339))
		require.Error(t, err)
	})

	t.Run("parse empty string", func(t *testing.T) {
		_, _, _, _, _, err := ParseDuration("")
		require.Error(t, err)
	})

	t.Run("invalid ISO8601 duration", func(t *testing.T) {
		// Doesn't start with P
		_, _, _, _, _, err := ParseDuration("10D1M")
		require.Error(t, err)

		// Invalid formats
		_, _, _, _, _, err = ParseDuration("P")
		require.Error(t, err)
		_, _, _, _, _, err = ParseDuration("PM")
		require.Error(t, err)
		_, _, _, _, _, err = ParseDuration("PT1D")
		require.Error(t, err)
		_, _, _, _, _, err = ParseDuration("P_D")
		require.Error(t, err)
		_, _, _, _, _, err = ParseDuration("PTxS")
		require.Error(t, err)
	})
}

func TestParseTime(t *testing.T) {
	t.Run("parse time.Duration without offset", func(t *testing.T) {
		expected := time.Now().Add(30 * time.Minute)
		tm, err := ParseTime("0h30m0s", nil)
		require.NoError(t, err)
		assert.LessOrEqual(t, tm.Sub(expected), time.Second*2)
	})
	t.Run("parse time.Duration with offset", func(t *testing.T) {
		now := time.Now()
		offs := 5 * time.Second
		start := now.Add(offs)
		expected := start.Add(30 * time.Minute)
		tm, err := ParseTime("0h30m0s", &start)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), expected.Sub(tm))
	})
	t.Run("parse ISO 8601 duration with repetition", func(t *testing.T) {
		_, err := ParseTime("R5/PT30M", nil)
		require.Error(t, err)
	})
	t.Run("parse ISO 8601 duration without repetition", func(t *testing.T) {
		now, _ := time.Parse("2006-01-02 15:04:05", "2021-12-06 17:43:46")
		offs := 5 * time.Second
		start := now.Add(offs)
		expected := start.Add(time.Hour*24*31 + time.Hour*2 + time.Minute*10 + time.Second*3)
		tm, err := ParseTime("P1MT2H10M3S", &start)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), expected.Sub(tm))
	})
	t.Run("parse RFC3339 datetime", func(t *testing.T) {
		dummy := time.Now().Add(5 * time.Minute)
		expected := time.Now().Truncate(time.Minute).Add(time.Minute)
		tm, err := ParseTime(expected.Format(time.RFC3339), &dummy)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), expected.Sub(tm))
	})
	t.Run("parse RFC3339nano datetime", func(t *testing.T) {
		dummy := time.Now().Add(1000 * time.Nanosecond)
		expected := time.Now().Add(100 * time.Nanosecond)
		tm, err := ParseTime(expected.Format(time.RFC3339Nano), &dummy)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), expected.Sub(tm))
	})
	t.Run("parse empty string", func(t *testing.T) {
		_, err := ParseTime("", nil)
		require.ErrorContains(t, err, "unsupported time/duration format")
	})
}
