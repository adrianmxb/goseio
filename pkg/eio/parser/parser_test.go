package parser

import (
	"fmt"
	"math"
	"strconv"
	"testing"
)

func BenchmarkMethod1(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		var t []byte
		strconv.AppendInt(t, int64(n), 10)
	}
}

var zero []byte = []byte{'0'}

func BenchmarkMethod2(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		if n <= 0 {
			continue
		}
		t := make([]byte, int(math.Log10(float64(n)))+1)
		nn := n
		for i := 0; i < len(t); i++ {
			t[i] = byte(nn%10) + '0'
			nn /= 10
		}
	}
}

func BenchmarkMethod21(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		var t []byte
		nn := n

		for nn > 0 {
			t = append(t, byte(nn%10)+'0')
			nn /= 10
		}
	}
}

func BenchmarkMethod3(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		var _ []byte
		_ = []byte(fmt.Sprintf("%d:", n))
	}
}
