package touch

import (
	"testing"
	"time"

	_ "time/tzdata"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

func TestParseTime(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		value string
		want  time.Time
	}{
		{"11061430", time.Date(2026, 11, 6, 14, 30, 0, 0, time.UTC)},
		{"11061430.07", time.Date(2026, 11, 6, 14, 30, 7, 0, time.UTC)},
		{"2611061430.15", time.Date(2026, 11, 6, 14, 30, 15, 0, time.UTC)},
		{"0001010000", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"202402291200", time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)},
		{"202611061430.59", time.Date(2026, 11, 6, 14, 30, 59, 0, time.UTC)},
		{"6801011200", time.Date(2068, 1, 1, 12, 0, 0, 0, time.UTC)},
		{"6901011200", time.Date(1969, 1, 1, 12, 0, 0, 0, time.UTC)},
		{"9912312359", time.Date(1999, 12, 31, 23, 59, 0, 0, time.UTC)},
		{"196001010000", time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseTime(tt.value, now)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseTimeInvalid(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"too short", "1106143"},
		{"9 digits", "110614301"},
		{"too long", "20261106143000"},
		{"sign", "+11061430"},
		{"sign", "-11061430"},
		{"whitespace", " 11061430"},
		{"whitespace", "11061430 "},
		{"whitespace", "1106 1430"},
		{"February 30", "202602300900"},
		{"not a leap year", "202502290900"},
		{"month 13", "202613011200"},
		{"month 0", "202600011200"},
		{"day 0", "202611001200"},
		{"hour 24", "202611062400"},
		{"minute 60", "202611061260"},
		{"leap second", "202611061430.60"},
		{"one second digit", "202611061430.1"},
		{"202611061430.123", "202611061430.123"},
		{"202611061430.", "202611061430."},
		{"202611061430.ab", "202611061430.ab"},
		{".15", ".15"},
		{"full-width digits", "１１０６１４３０"},
		{"Arabic-Indic digits", "١١٠٦١٤٣٠"},
		{"year 0 cannot be stored", "000001010000"},
		{"beyond the update API's range", "999912312359"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := ParseTime(tt.value, now); err == nil {
				t.Fatalf("got %s, want an error", got)
			}
		})
	}
}

func TestParseTimeYearBoundary(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"december", time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"january", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTime("01010000", tt.now)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseTimeLocalZone(t *testing.T) {
	zone := time.FixedZone("UTC+9", 9*60*60)
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, zone)
	got, err := ParseTime("202611061430", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 6, 5, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestParseTimeDST(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, zone)

	t.Run("gap", func(t *testing.T) {
		if got, err := ParseTime("202603080230", now); err == nil {
			t.Fatalf("got %s, want an error for a nonexistent local time", got)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		got, err := ParseTime("202611010130", now)
		if err != nil {
			t.Fatal(err)
		}
		edt := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
		est := time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC)
		if !got.Equal(edt) && !got.Equal(est) {
			t.Fatalf("got %s, want 01:30 EDT (%s) or 01:30 EST (%s)", got, edt, est)
		}
	})
}

func TestValidateSelection(t *testing.T) {
	ordinary := time.Date(2026, 11, 6, 14, 30, 0, 0, time.UTC)
	far := time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)
	both := Options{}
	access := Options{Access: true}
	modify := Options{Modify: true}
	accessModify := Options{Access: true, Modify: true}

	tests := []struct {
		name    string
		times   filetime.Times
		opts    Options
		wantErr bool
	}{
		{"ordinary default", filetime.Times{Access: ordinary, Modify: ordinary}, both, false},
		{"far access default", filetime.Times{Access: far, Modify: ordinary}, both, true},
		{"far access both", filetime.Times{Access: far, Modify: ordinary}, accessModify, true},
		{"far access unselected", filetime.Times{Access: far, Modify: ordinary}, modify, false},
		{"far access selected", filetime.Times{Access: far, Modify: ordinary}, access, true},
		{"far modify default", filetime.Times{Access: ordinary, Modify: far}, both, true},
		{"far modify unselected", filetime.Times{Access: ordinary, Modify: far}, access, false},
		{"far modify selected", filetime.Times{Access: ordinary, Modify: far}, modify, true},
		{"zero access selected", filetime.Times{Modify: ordinary}, access, true},
		{"zero access unselected", filetime.Times{Modify: ordinary}, modify, false},
		{"zero modify selected", filetime.Times{Access: ordinary}, modify, true},
		{"pre-1601", filetime.Times{Access: time.Date(1500, 1, 1, 0, 0, 0, 0, time.UTC), Modify: ordinary}, both, true},
		{"latest valid", filetime.Times{Access: time.Unix(0, 1<<63-1), Modify: ordinary}, both, false},
		{"earliest valid", filetime.Times{Access: time.Unix(0, -1<<63+1), Modify: ordinary}, both, false},
		{"just past latest", filetime.Times{Access: time.Unix(0, 1<<63-1).Add(time.Nanosecond), Modify: ordinary}, both, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.times, tt.opts)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("got error %v, want error %t", err, tt.wantErr)
			}
		})
	}
}
