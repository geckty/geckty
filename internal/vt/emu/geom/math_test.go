package geom

import "testing"

func TestAbs(t *testing.T) {
	cases := map[int]int{5: 5, -5: 5, 0: 0, -1: 1}
	for in, want := range cases {
		if got := Abs(in); got != want {
			t.Errorf("Abs(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestMax(t *testing.T) {
	if got := Max(3, 5); got != 5 {
		t.Errorf("Max(3,5) = %d, want 5", got)
	}
	if got := Max(5, 3); got != 5 {
		t.Errorf("Max(5,3) = %d, want 5", got)
	}
	if got := Max(4, 4); got != 4 {
		t.Errorf("Max(4,4) = %d, want 4", got)
	}
}

func TestMax16(t *testing.T) {
	if got := Max16(int16(3), int16(5)); got != 5 {
		t.Errorf("Max16(3,5) = %d, want 5", got)
	}
	if got := Max16(int16(5), int16(3)); got != 5 {
		t.Errorf("Max16(5,3) = %d, want 5", got)
	}
	if got := Max16(int16(4), int16(4)); got != 4 {
		t.Errorf("Max16(4,4) = %d, want 4", got)
	}
}

func TestMax32(t *testing.T) {
	if got := Max32(int32(3), int32(5)); got != 5 {
		t.Errorf("Max32(3,5) = %d, want 5", got)
	}
	if got := Max32(int32(5), int32(3)); got != 5 {
		t.Errorf("Max32(5,3) = %d, want 5", got)
	}
	if got := Max32(int32(4), int32(4)); got != 4 {
		t.Errorf("Max32(4,4) = %d, want 4 (tie should return first per > not >=)", got)
	}
}

func TestMin(t *testing.T) {
	if got := Min(3, 5); got != 3 {
		t.Errorf("Min(3,5) = %d, want 3", got)
	}
	if got := Min(5, 3); got != 3 {
		t.Errorf("Min(5,3) = %d, want 3", got)
	}
	if got := Min(4, 4); got != 4 {
		t.Errorf("Min(4,4) = %d, want 4", got)
	}
}

func TestClamp(t *testing.T) {
	if got := Clamp(5, 0, 10); got != 5 {
		t.Errorf("Clamp(5,0,10) = %d, want 5", got)
	}
	if got := Clamp(-5, 0, 10); got != 0 {
		t.Errorf("Clamp(-5,0,10) = %d, want 0", got)
	}
	if got := Clamp(15, 0, 10); got != 10 {
		t.Errorf("Clamp(15,0,10) = %d, want 10", got)
	}
	if got := Clamp(0, 0, 10); got != 0 {
		t.Errorf("Clamp(0,0,10) = %d, want 0 (lower bound inclusive)", got)
	}
	if got := Clamp(10, 0, 10); got != 10 {
		t.Errorf("Clamp(10,0,10) = %d, want 10 (upper bound inclusive)", got)
	}
}

func TestAsUint16(t *testing.T) {
	if got := AsUint16(100); got != 100 {
		t.Errorf("AsUint16(100) = %d, want 100", got)
	}
	if got := AsUint16(-1); got != 0 {
		t.Errorf("AsUint16(-1) = %d, want 0 (clamped)", got)
	}
	if got := AsUint16(70000); got != 65535 {
		t.Errorf("AsUint16(70000) = %d, want 65535 (clamped to MaxUint16)", got)
	}
	if got := AsUint16(65535); got != 65535 {
		t.Errorf("AsUint16(65535) = %d, want 65535 (exact max, not clamped away)", got)
	}
}
