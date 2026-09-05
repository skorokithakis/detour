package main

import "testing"

func TestSelectBackendIndex(t *testing.T) {
	tests := []struct {
		name      string
		counts    []int
		threshold int
		want      int
	}{
		{
			// The preferred backend has a single success, so it is flapping,
			// while the second backend has been up the whole time.
			name:      "flapping backend never preempts a stable one",
			counts:    []int{1, 3},
			threshold: 3,
			want:      1,
		},
		{
			// The preferred backend is two checks into its recovery, which is
			// not enough to take the active slot back from the stable one.
			name:      "recovering backend below threshold does not reclaim the active slot",
			counts:    []int{2, 10},
			threshold: 3,
			want:      1,
		},
		{
			name:      "stable backend regains active after 3 checks",
			counts:    []int{3, 10},
			threshold: 3,
			want:      0,
		},
		{
			// After startup no streak can have reached the threshold yet, so
			// the first success must be enough.
			name:      "cold start picks on first success",
			counts:    []int{1, 0},
			threshold: 3,
			want:      0,
		},
		{
			// After a total outage every streak is reset, so recovery also
			// picks the first backend to succeed rather than waiting.
			name:      "total outage recovers on first success",
			counts:    []int{0, 1},
			threshold: 3,
			want:      1,
		},
		{
			// A threshold of 1 must reproduce the old behaviour exactly: the
			// first backend with a live result wins.
			name:      "threshold of 1 selects the first backend with a success",
			counts:    []int{0, 1, 0},
			threshold: 1,
			want:      1,
		},
		{
			name:      "no backend has succeeded",
			counts:    []int{0, 0},
			threshold: 3,
			want:      -1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectBackendIndex(test.counts, test.threshold); got != test.want {
				t.Errorf("selectBackendIndex(%v, %d) = %d, want %d", test.counts, test.threshold, got, test.want)
			}
		})
	}
}
