// Copyright 2014 Google Inc.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansi

import "testing"

func TestSGRCodes(t *testing.T) {
	for _, tt := range []struct {
		got, want int
	}{
		{Reserved26, 26},
		{Reserved38, 38},
		{Reserved48, 48},
		{Reserved50, 50},
		{Reserved56, 56},
		{Reserved57, 57},
		{Reserved58, 58},
		{Reserved59, 59},
		{reserved66, 66},
	} {
		if tt.got != tt.want {
			t.Errorf("got %d, want %d\n", tt.got, tt.want)
		}
	}
}
