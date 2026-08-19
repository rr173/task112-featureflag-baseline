package eval

import "testing"

func TestRolloutBucketDeterministic(t *testing.T) {
	a := rolloutBucket("flagX", "userA")
	b := rolloutBucket("flagX", "userA")
	if a != b {
		t.Fatalf("rollout bucket not deterministic: %d vs %d", a, b)
	}
	if a < 0 || a >= 100 {
		t.Fatalf("rollout bucket out of range: %d", a)
	}
	// 不同 target 大概率不同桶（至少不全相同）。
	seen := map[int]bool{}
	for i := 0; i < 50; i++ {
		seen[rolloutBucket("flagX", string(rune('a'+i)))] = true
	}
	if len(seen) < 10 {
		t.Fatalf("rollout buckets insufficiently spread: %d distinct", len(seen))
	}
}

func TestRolloutBucketRange(t *testing.T) {
	for i := 0; i < 200; i++ {
		b := rolloutBucket("f", string(rune(i)))
		if b < 0 || b >= 100 {
			t.Fatalf("bucket %d out of [0,100)", b)
		}
	}
}
