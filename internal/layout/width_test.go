package layout

import "testing"

func TestExcelWidthPositive(t *testing.T) {
	if w := ExcelWidthChars("", false); w <= 0 {
		t.Fatalf("w=%v", w)
	}
	if w := ExcelWidthChars("Hello World", false); w <= ExcelWidthChars("Hi", false) {
		t.Fatal("longer text must be wider")
	}
}

func TestExcelWidthCap(t *testing.T) {
	s := ""
	for i := 0; i < 500; i++ {
		s += "W"
	}
	if w := ExcelWidthChars(s, false); w > 255 {
		t.Fatalf("w=%v > 255", w)
	}
}

func TestWordColsSum(t *testing.T) {
	for _, n := range []int{1, 2, 3, 5} {
		ws := WordTableColWidths(9360, n)
		if len(ws) != n {
			t.Fatalf("n=%d len=%d", n, len(ws))
		}
		sum := 0
		for _, w := range ws {
			sum += w
		}
		if sum != 9360 {
			t.Fatalf("n=%d sum=%d", n, sum)
		}
	}
}
