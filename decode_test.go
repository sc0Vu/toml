package toml

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDecodeSimple(t *testing.T) {
	var testSimple = `
age = 250
andrew = "gallant"
kait = "brady"
now = 1987-07-05T05:45:00Z
nowEast = 2017-06-22T16:15:21+08:00
nowWest = 2017-06-22T02:14:36-06:00
yesOrNo = true
pi = 3.14
colors = [
	["red", "green", "blue"],
	["cyan", "magenta", "yellow", "black"],
]

[My.Cats]
plato = "cat 1"
cauchy = "cat 2"
`

	type cats struct {
		Plato  string
		Cauchy string
	}
	type simple struct {
		Age     int
		Colors  [][]string
		Pi      float64
		YesOrNo bool
		Now     time.Time
		NowEast time.Time
		NowWest time.Time
		Andrew  string
		Kait    string
		My      map[string]cats
	}

	var val simple
	_, err := Decode(testSimple, &val)
	if err != nil {
		t.Fatal(err)
	}

	now, err := time.Parse("2006-01-02T15:04:05", "1987-07-05T05:45:00")
	if err != nil {
		panic(err)
	}
	nowEast, err := time.Parse("2006-01-02T15:04:05-07:00", "2017-06-22T16:15:21+08:00")
	if err != nil {
		panic(err)
	}
	nowWest, err := time.Parse("2006-01-02T15:04:05-07:00", "2017-06-22T02:14:36-06:00")
	if err != nil {
		panic(err)
	}
	var answer = simple{
		Age:     250,
		Andrew:  "gallant",
		Kait:    "brady",
		Now:     now,
		NowEast: nowEast,
		NowWest: nowWest,
		YesOrNo: true,
		Pi:      3.14,
		Colors: [][]string{
			{"red", "green", "blue"},
			{"cyan", "magenta", "yellow", "black"},
		},
		My: map[string]cats{
			"Cats": {Plato: "cat 1", Cauchy: "cat 2"},
		},
	}
	if !reflect.DeepEqual(val, answer) {
		t.Fatalf("\nwant: %#v\nhave: %#v", answer, val)
	}
}

func TestUTF16(t *testing.T) {
	tests := []struct {
		in      []byte
		wantErr string
	}{
		// a = "b" in UTF-16, without BOM and with the LE and BE BOMs.
		{
			[]byte{0x61, 0x00, 0x20, 0x00, 0x3d, 0x00, 0x20, 0x00, 0x22, 0x00, 0x62, 0x00, 0x22, 0x00, 0x0a, 0x00},
			`files cannot contain NULL bytes; probably using UTF-16; TOML files must be UTF-8`,
		},
		{
			[]byte{0xfe, 0xff, 0x61, 0x00, 0x20, 0x00, 0x3d, 0x00, 0x20, 0x00, 0x22, 0x00, 0x62, 0x00, 0x22, 0x00, 0x0a, 0x00},
			`files cannot contain NULL bytes; probably using UTF-16; TOML files must be UTF-8`,
		},
		//  UTF-8 with BOM
		{[]byte("\xff\xfea = \"b\""), ``},
		{[]byte("\xfe\xffa = \"b\""), ``},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var s struct{ A string }

			_, err := Decode(string(tt.in), &s)
			if !errorContains(err, tt.wantErr) {
				t.Fatalf("wrong error\nhave: %q\nwant: %q", err, tt.wantErr)
			}
			if tt.wantErr != "" {
				return
			}
			if s.A != "b" {
				t.Errorf(`s.A is not "b" but %q`, s.A)
			}
		})
	}
}

func TestDecodeEmbedded(t *testing.T) {
	type Dog struct{ Name string }
	type Age int
	type cat struct{ Name string }

	for _, test := range []struct {
		label       string
		input       string
		decodeInto  interface{}
		wantDecoded interface{}
	}{
		{
			label:       "embedded struct",
			input:       `Name = "milton"`,
			decodeInto:  &struct{ Dog }{},
			wantDecoded: &struct{ Dog }{Dog{"milton"}},
		},
		{
			label:       "embedded non-nil pointer to struct",
			input:       `Name = "milton"`,
			decodeInto:  &struct{ *Dog }{},
			wantDecoded: &struct{ *Dog }{&Dog{"milton"}},
		},
		{
			label:       "embedded nil pointer to struct",
			input:       ``,
			decodeInto:  &struct{ *Dog }{},
			wantDecoded: &struct{ *Dog }{nil},
		},
		{
			label:       "unexported embedded struct",
			input:       `Name = "socks"`,
			decodeInto:  &struct{ cat }{},
			wantDecoded: &struct{ cat }{cat{"socks"}},
		},
		{
			label:       "embedded int",
			input:       `Age = -5`,
			decodeInto:  &struct{ Age }{},
			wantDecoded: &struct{ Age }{-5},
		},
	} {
		_, err := Decode(test.input, test.decodeInto)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(test.wantDecoded, test.decodeInto) {
			t.Errorf("%s: want decoded == %+v, got %+v",
				test.label, test.wantDecoded, test.decodeInto)
		}
	}
}

func TestDecodeIgnoredFields(t *testing.T) {
	type simple struct {
		Number int `toml:"-"`
	}
	const input = `
Number = 123
- = 234
`
	var s simple
	if _, err := Decode(input, &s); err != nil {
		t.Fatal(err)
	}
	if s.Number != 0 {
		t.Errorf("got: %d; want 0", s.Number)
	}
}

func TestTableArrays(t *testing.T) {
	var tomlTableArrays = `
[[albums]]
name = "Born to Run"

  [[albums.songs]]
  name = "Jungleland"

  [[albums.songs]]
  name = "Meeting Across the River"

[[albums]]
name = "Born in the USA"

  [[albums.songs]]
  name = "Glory Days"

  [[albums.songs]]
  name = "Dancing in the Dark"
`

	type Song struct {
		Name string
	}

	type Album struct {
		Name  string
		Songs []Song
	}

	type Music struct {
		Albums []Album
	}

	expected := Music{[]Album{
		{"Born to Run", []Song{{"Jungleland"}, {"Meeting Across the River"}}},
		{"Born in the USA", []Song{{"Glory Days"}, {"Dancing in the Dark"}}},
	}}
	var got Music
	if _, err := Decode(tomlTableArrays, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("\n%#v\n!=\n%#v\n", expected, got)
	}
}

func TestTableNesting(t *testing.T) {
	for _, tt := range []struct {
		t    string
		want []string
	}{
		{"[a.b.c]", []string{"a", "b", "c"}},
		{`[a."b.c"]`, []string{"a", "b.c"}},
		{`[a.'b.c']`, []string{"a", "b.c"}},
		{`[a.' b ']`, []string{"a", " b "}},
		{"[ d.e.f ]", []string{"d", "e", "f"}},
		{"[ g . h . i ]", []string{"g", "h", "i"}},
		{`[ j . "ʞ" . 'l' ]`, []string{"j", "ʞ", "l"}},
	} {
		var m map[string]interface{}
		if _, err := Decode(tt.t, &m); err != nil {
			t.Errorf("Decode(%q): got error: %s", tt.t, err)
			continue
		}
		if keys := extractNestedKeys(m); !reflect.DeepEqual(keys, tt.want) {
			t.Errorf("Decode(%q): got nested keys %#v; want %#v",
				tt.t, keys, tt.want)
		}
	}
}

func extractNestedKeys(v map[string]interface{}) []string {
	var result []string
	for {
		if len(v) != 1 {
			return result
		}
		for k, m := range v {
			result = append(result, k)
			var ok bool
			v, ok = m.(map[string]interface{})
			if !ok {
				return result
			}
		}

	}
}

// Case insensitive matching tests.
// A bit more comprehensive than needed given the current implementation,
// but implementations change.
// Probably still missing demonstrations of some ugly corner cases regarding
// case insensitive matching and multiple fields.
func TestCase(t *testing.T) {
	var caseToml = `
tOpString = "string"
tOpInt = 1
tOpFloat = 1.1
tOpBool = true
tOpdate = 2006-01-02T15:04:05Z
tOparray = [ "array" ]
Match = "i should be in Match only"
MatcH = "i should be in MatcH only"
once = "just once"
[nEst.eD]
nEstedString = "another string"
`

	type InsensitiveEd struct {
		NestedString string
	}

	type InsensitiveNest struct {
		Ed InsensitiveEd
	}

	type Insensitive struct {
		TopString string
		TopInt    int
		TopFloat  float64
		TopBool   bool
		TopDate   time.Time
		TopArray  []string
		Match     string
		MatcH     string
		Once      string
		OncE      string
		Nest      InsensitiveNest
	}

	tme, err := time.Parse(time.RFC3339, time.RFC3339[:len(time.RFC3339)-5])
	if err != nil {
		panic(err)
	}
	expected := Insensitive{
		TopString: "string",
		TopInt:    1,
		TopFloat:  1.1,
		TopBool:   true,
		TopDate:   tme,
		TopArray:  []string{"array"},
		MatcH:     "i should be in MatcH only",
		Match:     "i should be in Match only",
		Once:      "just once",
		OncE:      "",
		Nest: InsensitiveNest{
			Ed: InsensitiveEd{NestedString: "another string"},
		},
	}
	var got Insensitive
	if _, err := Decode(caseToml, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("\n%#v\n!=\n%#v\n", expected, got)
	}
}

func TestPointers(t *testing.T) {
	type Object struct {
		Type        string
		Description string
	}

	type Dict struct {
		NamedObject map[string]*Object
		BaseObject  *Object
		Strptr      *string
		Strptrs     []*string
	}
	s1, s2, s3 := "blah", "abc", "def"
	expected := &Dict{
		Strptr:  &s1,
		Strptrs: []*string{&s2, &s3},
		NamedObject: map[string]*Object{
			"foo": {"FOO", "fooooo!!!"},
			"bar": {"BAR", "ba-ba-ba-ba-barrrr!!!"},
		},
		BaseObject: &Object{"BASE", "da base"},
	}

	ex1 := `
Strptr = "blah"
Strptrs = ["abc", "def"]

[NamedObject.foo]
Type = "FOO"
Description = "fooooo!!!"

[NamedObject.bar]
Type = "BAR"
Description = "ba-ba-ba-ba-barrrr!!!"

[BaseObject]
Type = "BASE"
Description = "da base"
`
	dict := new(Dict)
	_, err := Decode(ex1, dict)
	if err != nil {
		t.Errorf("Decode error: %v", err)
	}
	if !reflect.DeepEqual(expected, dict) {
		t.Fatalf("\n%#v\n!=\n%#v\n", expected, dict)
	}
}

func TestDecodeDatetime(t *testing.T) {
	tz7 := time.FixedZone("", -3600*7)

	for _, tt := range []struct {
		in   string
		want time.Time
	}{
		// Offset datetime
		{"1979-05-27T07:32:00Z", time.Date(1979, 05, 27, 07, 32, 0, 0, time.UTC)},
		{"1979-05-27T07:32:00.999999Z", time.Date(1979, 05, 27, 07, 32, 0, 999999000, time.UTC)},
		{"1979-05-27T00:32:00-07:00", time.Date(1979, 05, 27, 00, 32, 0, 0, tz7)},
		{"1979-05-27T00:32:00.999999-07:00", time.Date(1979, 05, 27, 00, 32, 0, 999999000, tz7)},
		{"1979-05-27T00:32:00.24-07:00", time.Date(1979, 05, 27, 00, 32, 0, 240000000, tz7)},
		{"1979-05-27 07:32:00Z", time.Date(1979, 05, 27, 07, 32, 0, 0, time.UTC)},
		{"1979-05-27t07:32:00z", time.Date(1979, 05, 27, 07, 32, 0, 0, time.UTC)},

		// Local datetime; according to the spec this should be "without any
		// relation to an offset or timezone. It cannot be converted to an
		// instant in time without additional information. Conversion to an
		// instant, if required, is implementation-specific."
		//
		// Go doesn't supporting a time without a timezone, so use time.Local.
		{"1979-05-27T07:32:00", time.Date(1979, 05, 27, 07, 32, 0, 0, time.Local)},
		{"1979-05-27T07:32:00.999999", time.Date(1979, 05, 27, 07, 32, 0, 999999000, time.Local)},
		{"1979-05-27T07:32:00.25", time.Date(1979, 05, 27, 07, 32, 0, 250000000, time.Local)},

		{"1979-05-27", time.Date(1979, 05, 27, 0, 0, 0, 0, time.Local)},

		{"07:32:00", time.Date(0, 1, 1, 07, 32, 0, 0, time.Local)},
		{"07:32:00.999999", time.Date(0, 1, 1, 07, 32, 0, 999999000, time.Local)},

		// Make sure the space between the datetime and "#" isn't lexed.
		{"1979-05-27T07:32:12-07:00  # c", time.Date(1979, 05, 27, 07, 32, 12, 0, tz7)},
	} {
		t.Run(tt.in, func(t *testing.T) {
			var x struct{ D time.Time }
			input := "d = " + tt.in
			if _, err := Decode(input, &x); err != nil {
				t.Fatalf("got error: %s", err)
			}

			if h, w := x.D.Format(time.RFC3339Nano), tt.want.Format(time.RFC3339Nano); h != w {
				t.Errorf("\nhave: %s\nwant: %s", h, w)
			}
		})
	}
}

func TestDecodeBadDatetime(t *testing.T) {
	var x struct{ T time.Time }
	for _, s := range []string{
		"123",
		"1230",
		"2006-01-50T00:00:00Z",
		"2006-01-30T00:00",
		"2006-01-30T",
	} {
		input := "T = " + s
		if _, err := Decode(input, &x); err == nil {
			t.Errorf("Expected invalid DateTime error for %q", s)
		}
	}
}

type sphere struct {
	Center [3]float64
	Radius float64
}

func TestDecodeSimpleArray(t *testing.T) {
	var s1 sphere
	if _, err := Decode(`center = [0.0, 1.5, 0.0]`, &s1); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeArrayWrongSize(t *testing.T) {
	var s1 sphere
	if _, err := Decode(`center = [0.1, 2.3]`, &s1); err == nil {
		t.Fatal("Expected array type mismatch error")
	}
}

func TestDecodeLargeIntoSmallInt(t *testing.T) {
	type table struct {
		Value int8
	}
	var tab table
	if _, err := Decode(`value = 500`, &tab); err == nil {
		t.Fatal("Expected integer out-of-bounds error.")
	}
}

func TestDecodeSizedInts(t *testing.T) {
	type table struct {
		U8  uint8
		U16 uint16
		U32 uint32
		U64 uint64
		U   uint
		I8  int8
		I16 int16
		I32 int32
		I64 int64
		I   int
	}
	answer := table{1, 1, 1, 1, 1, -1, -1, -1, -1, -1}
	toml := `
	u8 = 1
	u16 = 1
	u32 = 1
	u64 = 1
	u = 1
	i8 = -1
	i16 = -1
	i32 = -1
	i64 = -1
	i = -1
	`
	var tab table
	if _, err := Decode(toml, &tab); err != nil {
		t.Fatal(err.Error())
	}
	if answer != tab {
		t.Fatalf("Expected %#v but got %#v", answer, tab)
	}
}

func TestDecodeInts(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want int64
	}{
		{"0", 0},
		{"0x0", 0},
		{"0x00", 0},
		{"0o0", 0},
		{"0o00", 0},
		{"0b0", 0},
		{"0b00", 0},
		{"+0", 0},
		{"-0", 0},
		{"+99", 99},
		{"-10", -10},
		{"1_234_567", 1234567},
		{"1_2_3_4", 1234},
		{"0xdead_BEEF", 0xdeadbeef},
		{"0b0_1_1_0", 0b0110},
		{"0o7_7_7", 0o777},
		{"0x12345", 0x12345},
		{"0x0987", 0x987},
		{"0b1101", 0xd},
		{"0o777", 0x1ff},
		{"-9_223_372_036_854_775_808", math.MinInt64},
		{"9_223_372_036_854_775_807", math.MaxInt64},
	} {
		var x struct{ N int64 }
		input := "n = " + tt.s
		if _, err := Decode(input, &x); err != nil {
			t.Errorf("Decode(%q): got error: %s", input, err)
			continue
		}
		if x.N != tt.want {
			t.Errorf("Decode(%q): got %d; want %d", input, x.N, tt.want)
		}
	}
}

func TestDecodeFloats(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want float64
	}{
		{"+0.0", 0},
		{"-0.0", 0},
		{"+1.0", 1},
		{"3.1415", 3.1415},
		{"-0.01", -0.01},
		{"0.1", 0.1},
		{"5e+22", 5e22},
		{"1e6", 1e6},
		{"1e06", 1e6},
		{"1e006", 1e6},
		{"-2E-2", -2e-2},
		{"6.626e-34", 6.626e-34},
		{"9_224_617.445_991_228_313", 9224617.445991228313},
		{"9_876.54_32e1_0", 9876.5432e10},
		{"inf", math.Inf(0)},
		{"+inf", math.Inf(1)},
		{"-inf", math.Inf(-1)},
		{"nan", math.NaN()},
		{"+nan", math.NaN()},
		{"-nan", math.NaN()},
	} {
		t.Run(tt.s, func(t *testing.T) {
			var x struct{ N float64 }
			input := "n = " + tt.s
			if _, err := Decode(input, &x); err != nil {
				t.Fatalf("got error: %s", err)
			}
			if math.IsNaN(tt.want) {
				if !math.IsNaN(x.N) {
					t.Errorf("not NaN: %f", x.N)
				}
				return
			}
			if x.N != tt.want {
				t.Errorf("got %f; want %f", x.N, tt.want)
			}
		})
	}
}

func TestDecodeMalformedNumbers(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want string
	}{
		{"++99", "expected a digit"},
		{"0..1", "must be followed by one or more digits"},
		{"0.1.2", "Invalid float value"},
		{"1e2.3", "Invalid float value"},
		{"1e2e3", "Invalid float value"},
		{"_123", "expected value"},
		{"123_", "surrounded by digits"},
		{"0b0_", "surrounded by digits"},
		{"1._23", "surrounded by digits"},
		{"1e__23", "surrounded by digits"},
		{"123.", "must be followed by one or more digits"},
		{"1.e2", "must be followed by one or more digits"},
		{"00", "cannot have leading zeroes"},
		{"01", "cannot have leading zeroes"},
		{"+01", "cannot have leading zeroes"},
		{"-01", "cannot have leading zeroes"},
		{"01.2", "cannot have leading zeroes"},
		{"-01.2", "cannot have leading zeroes"},
		{"+01.2", "cannot have leading zeroes"},
		{"0x_d00d", "not a hexidecimal number: '0x_'"},
		{"0b_0", "not a binary number: '0b_'"},
		{"0z", "but got 'z' instead"},
		{"+0x3", "cannot use sign with non-decimal numbers: '+0x'"},
		{"-0xf00", "cannot use sign with non-decimal numbers: '-0x'"},
		{"0B0", "got 'B' instead"},
		{"0X0", "got 'X' instead"},
		{"0O0", "got 'O' instead"},
		{"in", "expected value"},
		{"+in", "invalid float: '+in'"},
		{"-in", "invalid float: '-in'"},
		{"na", "expected value"},
		{"+na", "invalid float: '+na'"},
		{"-na", "invalid float: '-na'"},
		{"na_n", "expected value"},
		{"+i_inf", "invalid float: '+i'"},
	} {
		t.Run(tt.s, func(t *testing.T) {
			var x struct{ N interface{} }
			input := "n = " + tt.s
			_, err := Decode(input, &x)
			if err == nil {
				t.Fatalf("got nil, want error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("\nhave: %q\nwant: %q", err, tt.want)
			}
		})
	}
}

func TestDecodeTypes(t *testing.T) {
	type mystr string

	for _, tt := range []struct {
		v    interface{}
		want string
	}{
		{new(map[string]int64), ""},
		{new(map[mystr]int64), ""},

		{3, "non-pointer int"},
		{(*int)(nil), "nil"},
		{new(map[int]string), "cannot decode to a map with non-string key type"},
		{new(map[interface{}]string), "cannot decode to a map with non-string key type"},
	} {
		t.Run(fmt.Sprintf("%T", tt.v), func(t *testing.T) {
			_, err := Decode(`x = 3`, tt.v)
			if !errorContains(err, tt.want) {
				t.Errorf("wrong error\nhave: %q\nwant: %q", err, tt.want)
			}
		})
	}
}

func TestUnmarshaler(t *testing.T) {

	var tomlBlob = `
[dishes.hamboogie]
name = "Hamboogie with fries"
price = 10.99

[[dishes.hamboogie.ingredients]]
name = "Bread Bun"

[[dishes.hamboogie.ingredients]]
name = "Lettuce"

[[dishes.hamboogie.ingredients]]
name = "Real Beef Patty"

[[dishes.hamboogie.ingredients]]
name = "Tomato"

[dishes.eggsalad]
name = "Egg Salad with rice"
price = 3.99

[[dishes.eggsalad.ingredients]]
name = "Egg"

[[dishes.eggsalad.ingredients]]
name = "Mayo"

[[dishes.eggsalad.ingredients]]
name = "Rice"
`
	m := &menu{}
	if _, err := Decode(tomlBlob, m); err != nil {
		t.Fatal(err)
	}

	if len(m.Dishes) != 2 {
		t.Log("two dishes should be loaded with UnmarshalTOML()")
		t.Errorf("expected %d but got %d", 2, len(m.Dishes))
	}

	eggSalad := m.Dishes["eggsalad"]
	if _, ok := interface{}(eggSalad).(dish); !ok {
		t.Errorf("expected a dish")
	}

	if eggSalad.Name != "Egg Salad with rice" {
		t.Errorf("expected the dish to be named 'Egg Salad with rice'")
	}

	if len(eggSalad.Ingredients) != 3 {
		t.Log("dish should be loaded with UnmarshalTOML()")
		t.Errorf("expected %d but got %d", 3, len(eggSalad.Ingredients))
	}

	found := false
	for _, i := range eggSalad.Ingredients {
		if i.Name == "Rice" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Rice was not loaded in UnmarshalTOML()")
	}

	// test on a value - must be passed as *
	o := menu{}
	if _, err := Decode(tomlBlob, &o); err != nil {
		t.Fatal(err)
	}

}

func TestDecodeInlineTable(t *testing.T) {
	input := `
[CookieJar]
Types = {Chocolate = "yummy", Oatmeal = "best ever"}

[Seasons]
Locations = {NY = {Temp = "not cold", Rating = 4}, MI = {Temp = "freezing", Rating = 9}}
`
	type cookieJar struct {
		Types map[string]string
	}
	type properties struct {
		Temp   string
		Rating int
	}
	type seasons struct {
		Locations map[string]properties
	}
	type wrapper struct {
		CookieJar cookieJar
		Seasons   seasons
	}
	var got wrapper

	meta, err := Decode(input, &got)
	if err != nil {
		t.Fatal(err)
	}
	want := wrapper{
		CookieJar: cookieJar{
			Types: map[string]string{
				"Chocolate": "yummy",
				"Oatmeal":   "best ever",
			},
		},
		Seasons: seasons{
			Locations: map[string]properties{
				"NY": {
					Temp:   "not cold",
					Rating: 4,
				},
				"MI": {
					Temp:   "freezing",
					Rating: 9,
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after decode, got:\n\n%#v\n\nwant:\n\n%#v", got, want)
	}
	if len(meta.keys) != 12 {
		t.Errorf("after decode, got %d meta keys; want 12", len(meta.keys))
	}
	if len(meta.types) != 12 {
		t.Errorf("after decode, got %d meta types; want 12", len(meta.types))
	}
}

func TestDecodeInlineTableArray(t *testing.T) {
	type point struct {
		X, Y, Z int
	}
	var got struct {
		Points []point
	}
	// Example inline table array from the spec.
	const in = `
points = [ { x = 1, y = 2, z = 3 },
           { x = 7, y = 8, z = 9 },
           { x = 2, y = 4, z = 8 } ]

`
	if _, err := Decode(in, &got); err != nil {
		t.Fatal(err)
	}
	want := []point{
		{X: 1, Y: 2, Z: 3},
		{X: 7, Y: 8, Z: 9},
		{X: 2, Y: 4, Z: 8},
	}
	if !reflect.DeepEqual(got.Points, want) {
		t.Errorf("got %#v; want %#v", got.Points, want)
	}
}

func TestDecodeMalformedInlineTable(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want string
	}{
		{"{,}", "unexpected comma"},
		{"{x = 3 y = 4}", "expected a comma or an inline table terminator"},
		{"{x=3,,y=4}", "unexpected comma"},
		{"{x=3,\ny=4}", "newlines not allowed"},
		{"{x=3\n,y=4}", "newlines not allowed"},
	} {
		var x struct{ A map[string]int }
		input := "a = " + tt.s
		_, err := Decode(input, &x)
		if err == nil {
			t.Errorf("Decode(%q): got nil, want error containing %q",
				input, tt.want)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Decode(%q): got %q, want error containing %q",
				input, err, tt.want)
		}
	}
}

type menu struct {
	Dishes map[string]dish
}

func (m *menu) UnmarshalTOML(p interface{}) error {
	m.Dishes = make(map[string]dish)
	data, _ := p.(map[string]interface{})
	dishes := data["dishes"].(map[string]interface{})
	for n, v := range dishes {
		if d, ok := v.(map[string]interface{}); ok {
			nd := dish{}
			nd.UnmarshalTOML(d)
			m.Dishes[n] = nd
		} else {
			return fmt.Errorf("not a dish")
		}
	}
	return nil
}

type dish struct {
	Name        string
	Price       float32
	Ingredients []ingredient
}

func (d *dish) UnmarshalTOML(p interface{}) error {
	data, _ := p.(map[string]interface{})
	d.Name, _ = data["name"].(string)
	d.Price, _ = data["price"].(float32)
	ingredients, _ := data["ingredients"].([]map[string]interface{})
	for _, e := range ingredients {
		n, _ := interface{}(e).(map[string]interface{})
		name, _ := n["name"].(string)
		i := ingredient{name}
		d.Ingredients = append(d.Ingredients, i)
	}
	return nil
}

type ingredient struct {
	Name string
}

func TestDecodeSlices(t *testing.T) {
	type T struct {
		S []string
	}
	for i, tt := range []struct {
		v     T
		input string
		want  T
	}{
		{T{}, "", T{}},
		{T{[]string{}}, "", T{[]string{}}},
		{T{[]string{"a", "b"}}, "", T{[]string{"a", "b"}}},
		{T{}, "S = []", T{[]string{}}},
		{T{[]string{}}, "S = []", T{[]string{}}},
		{T{[]string{"a", "b"}}, "S = []", T{[]string{}}},
		{T{}, `S = ["x"]`, T{[]string{"x"}}},
		{T{[]string{}}, `S = ["x"]`, T{[]string{"x"}}},
		{T{[]string{"a", "b"}}, `S = ["x"]`, T{[]string{"x"}}},
	} {
		if _, err := Decode(tt.input, &tt.v); err != nil {
			t.Errorf("[%d] %s", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.v, tt.want) {
			t.Errorf("[%d] got %#v; want %#v", i, tt.v, tt.want)
		}
	}
}

func TestDecodePrimitive(t *testing.T) {
	type S struct {
		P Primitive
	}
	type T struct {
		S []int
	}
	slicep := func(s []int) *[]int { return &s }
	arrayp := func(a [2]int) *[2]int { return &a }
	mapp := func(m map[string]int) *map[string]int { return &m }
	for i, tt := range []struct {
		v     interface{}
		input string
		want  interface{}
	}{
		// slices
		{slicep(nil), "", slicep(nil)},
		{slicep([]int{}), "", slicep([]int{})},
		{slicep([]int{1, 2, 3}), "", slicep([]int{1, 2, 3})},
		{slicep(nil), "P = [1,2]", slicep([]int{1, 2})},
		{slicep([]int{}), "P = [1,2]", slicep([]int{1, 2})},
		{slicep([]int{1, 2, 3}), "P = [1,2]", slicep([]int{1, 2})},

		// arrays
		{arrayp([2]int{2, 3}), "", arrayp([2]int{2, 3})},
		{arrayp([2]int{2, 3}), "P = [3,4]", arrayp([2]int{3, 4})},

		// maps
		{mapp(nil), "", mapp(nil)},
		{mapp(map[string]int{}), "", mapp(map[string]int{})},
		{mapp(map[string]int{"a": 1}), "", mapp(map[string]int{"a": 1})},
		{mapp(nil), "[P]\na = 2", mapp(map[string]int{"a": 2})},
		{mapp(map[string]int{}), "[P]\na = 2", mapp(map[string]int{"a": 2})},
		{mapp(map[string]int{"a": 1, "b": 3}), "[P]\na = 2", mapp(map[string]int{"a": 2, "b": 3})},

		// structs
		{&T{nil}, "[P]", &T{nil}},
		{&T{[]int{}}, "[P]", &T{[]int{}}},
		{&T{[]int{1, 2, 3}}, "[P]", &T{[]int{1, 2, 3}}},
		{&T{nil}, "[P]\nS = [1,2]", &T{[]int{1, 2}}},
		{&T{[]int{}}, "[P]\nS = [1,2]", &T{[]int{1, 2}}},
		{&T{[]int{1, 2, 3}}, "[P]\nS = [1,2]", &T{[]int{1, 2}}},
	} {
		var s S
		md, err := Decode(tt.input, &s)
		if err != nil {
			t.Errorf("[%d] Decode error: %s", i, err)
			continue
		}
		if err := md.PrimitiveDecode(s.P, tt.v); err != nil {
			t.Errorf("[%d] PrimitiveDecode error: %s", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.v, tt.want) {
			t.Errorf("[%d] got %#v; want %#v", i, tt.v, tt.want)
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		in      string
		wantErr string
		lastKey bool
	}{
		{`x`, "unexpected EOF; expected key separator '='", false},
		{`x  `, "unexpected EOF; expected key separator '='", false},
		{`x="`, `unexpected EOF; expected '"'`, true},
		{`x="""`, `unexpected EOF; expected '"""'`, true},
		{`x='`, `unexpected EOF; expected "'"`, true},
		{`x='''`, `unexpected EOF; expected "'''"`, true},
		{"x = ", "unexpected EOF; expected value", true},
		{"x = \n", "expected value but found '\\n' instead", true},

		// Cases found by fuzzing in #155 and #239
		{`""` + "\ufffd", "invalid UTF-8", false},
		{`""=` + "\ufffd", "invalid UTF-8", false},
		{`x="""`, "unexpected EOF", true},
		{`x = [{ key = 42 #`, "expected a comma or an inline table terminator", true},
		{`x = {a = 42 #`, "expected a comma or an inline table terminator '}', but got end of file instead", true},
		{`x = [42 #`, "expected a comma or array terminator ']', but got end of file instead", false},

		// Literal escape characters are not alllowed in any strings
		{`x = """` + "\r" + `"""`, `control characters are not allowed`, true},
		{`x = """` + "\x01" + `"""`, `control characters are not allowed`, true},
		{`x = '''` + "\r" + `'''`, `control characters are not allowed`, true},
		{`x = '''` + "\x01" + `'''`, `control characters are not allowed`, true},
		{`x = "` + "\r" + `"`, `control characters are not allowed`, true},
		{`x = "` + "\x01" + `"`, `control characters are not allowed`, true},
		{`x = '` + "\r" + `'`, `control characters are not allowed`, true},
		{`x = '` + "\x01" + `'`, `control characters are not allowed`, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var x struct{}
			_, err := Decode(tt.in, &x)
			if err == nil {
				t.Fatal("err is nil")
			}
			if !errorContains(err, tt.wantErr) {
				t.Errorf("wrong error\nhave: %q\nwant: %q", err, tt.wantErr)
			}
			if tt.lastKey && !errorContains(err, "last key parsed 'x") {
				t.Errorf("last key parsed not in error\nhave: %q\nwant: %q", err, "last key parsed 'x'")
			}
		})
	}
}

func TestDecodeMultilineNewlines(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// Note "NL" gets replaced by "\n" and "\r\n" (the tests are run twice);
		// this makes it easier to read and write these tests.

		{`x = """"""`, ``},
		{`x = """\NL"""`, ``},       // Empty string
		{`x = """\NL\NL\NL"""`, ``}, // Empty string

		{`x = """a\NL    u2222b"""`, `au2222b`},     // Remove all whitespace after \
		{`x = """a\NLNLNLu2222b"""`, `au2222b`},     // Remove all newlines
		{`x = """a  \NL    u2222b"""`, `a  u2222b`}, // Don't remove whitespace before \

		{`x = """a \ NLb"""`, `a b`}, // Allow any whitespace between \n and \
		{`x = """a  \ NL b"""`, `a  b`},
		{`x = """a \ 		    NLb"""`, `a b`},

		{`x="""a\NLu2222b"""`, `au2222b`},        // Ends in \ → remove
		{`x="""a\\NLu2222b"""`, `a\NLu2222b`},    // Ends in \\ → literal backslash, so keep NL.
		{`x="""a\\\NLu2222b"""`, `a\u2222b`},     // Ends in \\\ → backslash followed by NL escape, so remove.
		{`x="""a\\\\NLu2222b"""`, `a\\NLu2222b`}, // Ends in \\\\ → two lieral backslashes; keep NL

		{`x = """NLa b \n cNLd e fNL"""`, "a b \n c\nd e f\n"},
		{`x = """a b c\NL"""`, "a b c"},

		{`x = """NLThe quick brown \NLNLNLfox jumps over \NL    the lazy dog."""`,
			`The quick brown fox jumps over the lazy dog.`},
		{`x = """\NL        The quick brown \NLNLNL        fox jumps over \NL        the lazy dog.\NL        """`,
			`The quick brown fox jumps over the lazy dog.`},
	}

	replUnix := strings.NewReplacer("NL", "\n")
	replWin := strings.NewReplacer("NL", "\r\n")
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Run("unix", func(t *testing.T) {
				in := replUnix.Replace(tt.in)
				want := replUnix.Replace(tt.want)

				var s struct{ X string }
				_, err := Decode(in, &s)
				if err != nil {
					t.Fatal(err)
				}
				if s.X != want {
					t.Errorf("\nhave: %q\nwant: %q", s.X, want)
				}
			})

			t.Run("windows", func(t *testing.T) {
				in := replWin.Replace(tt.in)
				want := replWin.Replace(tt.want)

				var s struct{ X string }
				_, err := Decode(in, &s)
				if err != nil {
					t.Fatal(err)
				}
				if s.X != want {
					t.Errorf("\nhave: %q\nwant: %q", s.X, want)
				}
			})
		})
	}
}

// Test for https://github.com/BurntSushi/toml/pull/166.
func TestDecodeBoolArray(t *testing.T) {
	for _, tt := range []struct {
		s    string
		got  interface{}
		want interface{}
	}{
		{
			"a = [true, false]",
			&struct{ A []bool }{},
			&struct{ A []bool }{[]bool{true, false}},
		},
		{
			"a = {a = true, b = false}",
			&struct{ A map[string]bool }{},
			&struct{ A map[string]bool }{map[string]bool{"a": true, "b": false}},
		},
	} {
		if _, err := Decode(tt.s, tt.got); err != nil {
			t.Errorf("Decode(%q): %s", tt.s, err)
			continue
		}
		if !reflect.DeepEqual(tt.got, tt.want) {
			t.Errorf("Decode(%q): got %#v; want %#v", tt.s, tt.got, tt.want)
		}
	}
}

func BenchmarkDecode(b *testing.B) {
	var testSimple = `
age = 250
andrew = "gallant"
kait = "brady"
now = 1987-07-05T05:45:00Z
nowEast = 2017-06-22T16:15:21+08:00
nowWest = 2017-06-22T02:14:36-06:00
yesOrNo = true
pi = 3.14
colors = [
	["red", "green", "blue"],
	["cyan", "magenta", "yellow", "black"],
]

[My.Cats]
plato = "cat 1"
cauchy = """ cat 2
"""
`

	type cats struct {
		Plato  string
		Cauchy string
	}
	type simple struct {
		Age     int
		Colors  [][]string
		Pi      float64
		YesOrNo bool
		Now     time.Time
		NowEast time.Time
		NowWest time.Time
		Andrew  string
		Kait    string
		My      map[string]cats
	}

	var val simple
	_, err := Decode(testSimple, &val)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		Decode(testSimple, &val)
	}
}

func TestParseError(t *testing.T) {
	file :=
		`a = "a"
b = "b"
c = 001  # invalid
`

	var s struct {
		A, B string
		C    int
	}
	_, err := Decode(file, &s)
	if err == nil {
		t.Fatal("err is nil")
	}

	var pErr ParseError
	if !errors.As(err, &pErr) {
		t.Fatalf("err is not a ParseError: %T %[1]v", err)
	}

	want := ParseError{
		Line:    3,
		LastKey: "c",
		Message: `Invalid integer "001": cannot have leading zeroes`,
	}
	if !strings.Contains(pErr.Message, want.Message) ||
		pErr.Line != want.Line ||
		pErr.LastKey != want.LastKey {
		t.Errorf("unexpected data\nhave: %#v\nwant: %#v", pErr, want)
	}
}

// errorContains checks if the error message in have contains the text in
// want.
//
// This is safe when have is nil. Use an empty string for want if you want to
// test that err is nil.
func errorContains(have error, want string) bool {
	if have == nil {
		return want == ""
	}
	if want == "" {
		return false
	}
	return strings.Contains(have.Error(), want)
}

func TestDecodeDottedKey(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr string
	}{
		// Bare key
		{`a.b=1`, `{"a":{"b":1}}`, ``},
		{` a . b = 1`, `{"a":{"b":1}}`, ``},
		{" \t  a   \t  .   \t   b  \t = 1", `{"a":{"b":1}}`, ``},
		// Quoted key
		{`"a"."b"=1`, `{"a":{"b":1}}`, ``},
		{` "a" . "b" = 1`, `{"a":{"b":1}}`, ``},
		{" \t  'a'   \t  .   \t   'b'  \t = 1", `{"a":{"b":1}}`, ``},
		// Bare + quoted
		{`a."b"=1`, `{"a":{"b":1}}`, ``},
		{` a . "b" = 1`, `{"a":{"b":1}}`, ``},
		{" \t  a   \t  .   \t   'b'  \t = 1", `{"a":{"b":1}}`, ``},
		// Quoted + bare
		{`"a".b=1`, `{"a":{"b":1}}`, ``},
		{` "a" . b = 1`, `{"a":{"b":1}}`, ``},
		{" \t  'a'   \t  .   \t   b  \t = 1", `{"a":{"b":1}}`, ``},
		// Inside a table.
		{"[a.b]\nc=1", `{"a":{"b":{"c":1}}}`, ``},
		{"[a.b.c]\nc=1", `{"a":{"b":{"c":{"c":1}}}}`, ``},
		{"[[a.b]]\nc=1", `{"a":{"b":[{"c":1}]}}`, ``},
		{"[a.b]\nc.d=1", `{"a":{"b":{"c":{"d":1}}}}`, ``},
		{"[a.b.c.d]\ne.f.g.h=1", `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":1}}}}}}}}`, ``},
		{"[[a.b.x.y]]\nc.d.e.f=1", `{"a":{"b":{"x":{"y":[{"c":{"d":{"e":{"f":1}}}}]}}}}`, ``},

		// Multiple values on the same key
		{"a.b=1\na.c=2\na.d=[3]", `{"a":{"b":1,"c":2,"d":[3]}}`, ``},

		// Inline table.
		//{`a={b.c=1}`, ``, ``},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var s interface{}
			_, err := Decode(tt.in, &s)
			if err != nil {
				t.Fatal(err)
			}

			j, _ := json.Marshal(s)
			have := string(j)

			//if have := fmt.Sprintf("%v", s); have != tt.want {
			if have != tt.want {
				j, _ := json.MarshalIndent(s, "", "  ")
				t.Errorf("\nhave: %s\nwant: %s\n\nhave indented:\n%s", have, tt.want, j)
			}
		})
	}
}
